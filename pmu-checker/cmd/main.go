//###########################################################################################################
//# Copyright (C) 2021 Intel Corporation
//# SPDX-License-Identifier: BSD-3-Clause
//###########################################################################################################

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/intel/perfspect/pmu-checker/msr"
)

const (
	description = `pmu-checker

Allows us to verify if the system is running any drivers/daemons that may be programming the PMU.

Options:
`
)

var (
	loglevel       = flag.Bool("debug", false, "set the loglevel to debug, default is info")
	multiLogWriter = flag.Bool("no-stdout", false, "set the logwriter to write to logfile only, default is false")
	cpu            = flag.Int("cpu", 0, "Read MSRs on respective CPU, default is 0")
	logfile        = flag.String("logfile", "pmu-checker.log", "set the logfile name, default is pmu-checker.log")
	help           = flag.Bool("help", false, "Shows the usage of pmu-checker application")
)

func validateLogFileName(file string) {
	regexString := `([a-zA-Z0-9\s_\\.\-\(\):])+(.log|.txt)$`
	reg, err := regexp.Compile(regexString)
	if err != nil {
		log.Fatal(err)
		os.Exit(0)
	}

	if reg.MatchString(file) {
		return
	} else {
		log.Panic("The file name isn't valid for logging, The valid extensions are .log and .txt")
	}

}

func initialize() error {
	validateLogFileName(*logfile)

	log.SetFormatter(&log.TextFormatter{
		ForceColors:            true,
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})
	// programDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path of the program")
	}
	exPath := filepath.Dir(ex)
	file, err := os.OpenFile(filepath.Join(exPath, *logfile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.Wrap(err, "failed to open the log file")
	}

	//write to logfile and stdout at same time
	mw := io.MultiWriter(os.Stdout, file)

	if *multiLogWriter == true {
		log.SetOutput(file)
	} else {
		log.SetOutput(mw)
	}

	log.SetLevel(log.InfoLevel)
	if *loglevel == true {
		log.SetLevel(log.DebugLevel)
	}

	err = msr.Initialize()
	if err != nil {
		return errors.Wrap(err, "couldn't initialize msr module")
	}

	return nil
}

func main() {
	if os.Geteuid() != 0 {
		println("You need a root privileges to run.")
		os.Exit(2)
	}

	flag.Parse()

	if *help == true {
		println(description)
		flag.PrintDefaults()
		os.Exit(0)
	}

	err := initialize()
	if err != nil {
		log.Error(errors.Wrap(err, "couldn't initialize PMU Checker"))
		os.Exit(2)
	}

	err = msr.ValidateMSRModule(*cpu)
	if err != nil {
		log.Error(errors.Wrap(err, "couldn't validate MSR module"))
		os.Exit(2)
	}

	log.Info("Starting the PMU Checker application...")

	var wg sync.WaitGroup

	for i := 1; i <= 6; i++ {

		if len(msr.Del) == 7 {
			// if all the PMUs are being used, break the loop

			log.Infof("Aborting iteration check #%d", i)
			break
		}

		log.Debugf("Iteration check #%d started\n", i)
		msr.Values.Range(func(key, value interface{}) bool {
			_, ok := value.(uint64)
			if !ok {
				return false
			}

			wg.Add(1)
			go msr.ReadMSR(key.(string), &wg, i, *cpu)
			return true
		})

		wg.Wait()
		log.Infof("Iteration check #%d completed\n", i)
		//intentional sleep
		time.Sleep(time.Second)

	}

	res := new(Result)

	log.Infof(strings.Repeat("-", 12) + "All Iteration checks completed" + strings.Repeat("-", 12))
	res.PMUDetails = make(map[string]string)
	if len(msr.Del) == 0 {
		log.Infof("None of the PMU(s) are actively being used\n")
		res.PMUActive = 0
	}

	if len(msr.Del) > 0 {

		log.Info("Following PMU(s) are actively being used:")
		for i := 0; i < len(msr.Del); i++ {
			pmu := msr.Del[i]
			switch pmu {

			case "0x309":
				res.PMUDetails[pmu] = "instructions"
				log.Infof("%s: might be using instructions", pmu)
			case "0x30a":
				res.PMUDetails[pmu] = "cpu_cycles"
				log.Infof("%s: might be using cpu_cycles (check if nmi_watchdog is running)", pmu)
			case "0x30b":
				res.PMUDetails[pmu] = "ref_cycles"
				log.Infof("%s: might be using ref_cycles", pmu)
			case "0xc1", "0xc2", "0xc3", "0xc4":
				res.PMUDetails[pmu] = "General_purpose_programmable_PMU"
				log.Infof("%s: might be using general programmable PMU", pmu)
			default:
				// must not enter default case
				log.Infof("Report this to the Developers")
				os.Exit(2)

			}

		}
		res.PMUActive = len(msr.Del)
	}

	fmt.Println(res)
}

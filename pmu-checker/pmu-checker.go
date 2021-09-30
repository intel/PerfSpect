//###########################################################################################################
//# Copyright (C) 2021 Intel Corporation
//# SPDX-License-Identifier: BSD-3-Clause
//###########################################################################################################

package main

import (
	"encoding/json"
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

var (
	loglevel       = flag.Bool("debug", false, "set the loglevel to debug, default is info")
	multiLogWriter = flag.Bool("no-stdout", false, "set the logwriter to write to logfile only, default is false")
	cpu            = flag.Int("cpu", 0, "Read MSRs on respective CPU, default is 0")
	logfile        = flag.String("logfile", "pmu-checker.log", "set the logfile name, default is pmu-checker.log")
	help           = flag.Bool("help", false, "Shows the usage of pmu-checker application")
)

var CPU int

type Result struct {
	Pmu_active_count int               `json:"PMU(s)_active"`
	Pmu_details      map[string]string `json:"Details"`
}

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

func showUsage() {
	fmt.Println("pmu-checker needs to be run with sudo previlages")
	fmt.Println("Usage:")
	fmt.Println(" sudo ./pmu-checker [OPTION...]")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println(" logfile[*.log],  cpu[int], debug, no-stdout")

}

func initialize() error {
	//parse the commandline arguments
	flag.Parse()

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

	CPU = *cpu

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

	err := initialize()
	if err != nil {
		log.Error(errors.Wrap(err, "couldn't initialize PMU Checker"))
		return
	}

	if *help == true {
		showUsage()
		return
	}

	log.Info("Starting the PMU Checker application...")
	msr.ValidateMSRModule(CPU)

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
			go msr.ReadMSR(key.(string), &wg, i, CPU)
			return true
		})

		wg.Wait()
		log.Infof("Iteration check #%d completed\n", i)
		//intentional sleep
		time.Sleep(time.Second)

	}

	res := new(Result)

	log.Infof(strings.Repeat("-", 12) + "All Iteration checks completed" + strings.Repeat("-", 12))
	res.Pmu_details = make(map[string]string)
	if len(msr.Del) == 0 {
		log.Infof("None of the PMU(s) are actively being used\n")
		res.Pmu_active_count = 0
	}

	if len(msr.Del) > 0 {

		log.Info("Following PMU(s) are actively being used:")
		for i := 0; i < len(msr.Del); i++ {
			pmu := msr.Del[i]
			switch pmu {

			case "0x309":
				res.Pmu_details[pmu] = "instructions"
				log.Infof("%s: might be using instructions", pmu)
			case "0x30a":
				res.Pmu_details[pmu] = "cpu_cycles"
				log.Infof("%s: might be using cpu_cycles (check if nmi_watchdog is running)", pmu)
			case "0x30b":
				res.Pmu_details[pmu] = "ref_cycles"
				log.Infof("%s: might be using ref_cycles", pmu)
			case "0xc1", "0xc2", "0xc3", "0xc4":
				res.Pmu_details[pmu] = "General_purpose_programmable_PMU"
				log.Infof("%s: might be using general programmable PMU", pmu)
			default:
				// must not enter default case
				log.Infof("Report this to the Developers")
				os.Exit(0)

			}

		}
		res.Pmu_active_count = len(msr.Del)
	}

	var js []byte
	js, err = json.Marshal(res)
	if err != nil {
		log.Error(errors.Wrap(err, "result could not be converted to json"))
		return
	}
	fmt.Println(string(js))
}

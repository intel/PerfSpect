package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

//globals
var msrValues = sync.Map{}
var msrRegs = []string{
	"0x309",
	"0x30a",
	"0x30b",
	"0xc1",
	"0xc2",
	"0xc3",
	"0xc4",
}

var msrDel = []string{}
var CPU int

type retMSR struct {
	fd int
}
type Result struct {
	Pmu_active_count int               `json:"PMU(s)_active"`
	Pmu_details      map[string]string `json:"Details"`
}

const msrPath = "/dev/cpu/%d/msr"

func (dpt retMSR) Read(msr int64) (uint64, error) {
	// Reads a given MSR on the respective CPU

	buf := make([]byte, 8)
	rc, err := syscall.Pread(dpt.fd, buf, msr)
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	if rc != 8 {
		log.Errorf("wrong byte count %d", rc)
		return 0, fmt.Errorf("wrong byte count %d", rc)
	}

	//assuming all x86 uses little endian format
	msrVal := binary.LittleEndian.Uint64(buf)
	log.Tracef("MSR %d was read successfully as %d", msr, msrVal)
	return msrVal, err

}

func openMSRInterface(cpu int) (*retMSR, error) {
	// Open connection to MSR Interface with given cpu

	msrDir := fmt.Sprintf(msrPath, cpu)
	fd, err := syscall.Open(msrDir, syscall.O_RDONLY, 777)
	if err != nil {
		log.Errorf("Couln't open the msr interface, Error: ", err)
		return nil, errors.New("couldn't open the msr interface")
	}

	return &retMSR{fd: fd}, nil

}

func closeMSRInterface(dpt retMSR) {
	// Close connection to MSR Interface

	syscall.Close(dpt.fd)
}

func validateMSRModule(cpu int) {

	msrDir := fmt.Sprintf(msrPath, cpu)
	if _, err := os.Stat(msrDir); os.IsNotExist(err) {
		// if msr modules aren't loaded

		log.Panicf("MSR modules aren't loaded at %s, please load them using modprobe msr command\n", msrDir)
	}

}

func readMSR(reg string, wg *sync.WaitGroup, thread int, cpu int) {
	// Read MSR value, update map as needed

	defer wg.Done()
	log.Debugf("Worker %d starting %s", thread, reg)
	hexreg := strings.Replace(reg, "0x", "", -1)
	hexreg = strings.Replace(hexreg, "0X", "", -1)
	regInt64, err := strconv.ParseInt(hexreg, 16, 64)
	if err != nil {
		log.Panicf("The Hex to int64 type covertion failed\nError: ", err)
	}

	msr, err := openMSRInterface(cpu)
	if err != nil {
		log.Panic(err)
	}

	msrVal, err := msr.Read(regInt64)
	if err != nil {
		log.Panic(err)
	}

	closeMSRInterface(*msr)
	log.Debugf("New value of thread %d for %s is %d", thread, reg, msrVal)
	currentVal, found := msrValues.Load(reg)
	msrValues.Store(reg, msrVal)

	log.Debugf("Old value of thread %d for %s is %d", thread, reg, currentVal)

	if found && currentVal != uint64(0) && msrVal != currentVal {
		// The key exists but value has changed, delete it

		msrDel = append(msrDel, reg)
		log.Debugf("Deleting %s in the thread %d", reg, thread)
		msrValues.Delete(reg)

	}
	log.Debugf("Worker %d done for %s\n", thread, reg)

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

func init() {
	//parse the commandline arguments
	loglevel := flag.Bool("debug", false, "set the loglevel to debug, default is info")
	multiLogWriter := flag.Bool("no-stdout", false, "set the logwriter to write to logfile only, default is false")
	cpu := flag.Int("cpu", 0, "Read MSRs on respective CPU, default is 0")
	logfile := flag.String("logfile", "pmu-checker.log", "set the logfile name, default is pmu-checker.log")
	help := flag.Bool("help", false, "Shows the usage of pmu-checker application")

	flag.Parse()

	if *help {
		showUsage()
		os.Exit(0)
	}

	//don't allow non-sudo runs
	if os.Geteuid() != 0 {
		log.Fatalf("You need root privileges to run pmu-checker, please run again with sudo")
		os.Exit(0)
	}

	validateLogFileName(*logfile)

	log.SetFormatter(&log.TextFormatter{
		ForceColors:            true,
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})
	// programDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	ex, err := os.Executable()
	if err != nil {
		log.Errorf("Failed to get absolute path of the program\nError:", err)
	}
	exPath := filepath.Dir(ex)
	file, err := os.OpenFile(filepath.Join(exPath, *logfile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file\nError:", err)
	}

	//write to logfile and stdout at same time
	mw := io.MultiWriter(os.Stdout, file)

	if *multiLogWriter {
		log.SetOutput(file)
	} else {
		log.SetOutput(mw)
	}

	log.SetLevel(log.InfoLevel)
	if *loglevel {
		log.SetLevel(log.DebugLevel)
	}

	CPU = *cpu

}

func main() {

	log.Info("Starting the PMU Checker application...")
	validateMSRModule(CPU)

	// initialize the map
	for i := 0; i < len(msrRegs); i++ {
		msrValues.Store(msrRegs[i], uint64(0))
	}

	var wg sync.WaitGroup

	for i := 1; i <= 6; i++ {

		if len(msrDel) == 7 {
			// if all the PMUs are being used, break the loop

			log.Infof("Aborting iteration check #%d", i)
			break
		}

		log.Debugf("Iteration check #%d started\n", i)
		msrValues.Range(func(key, value interface{}) bool {
			_, ok := value.(uint64)
			if !ok {
				return false
			}

			wg.Add(1)
			go readMSR(key.(string), &wg, i, CPU)
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
	if len(msrDel) == 0 {
		log.Infof("None of the PMU(s) are actively being used\n")
		res.Pmu_active_count = 0
	}

	if len(msrDel) > 0 {

		log.Info("Following PMU(s) are actively being used:")
		for i := 0; i < len(msrDel); i++ {
			pmu := msrDel[i]
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
				// mustn't enter default case
				log.Infof("Report this to the Developers")
				os.Exit(0)

			}

		}
		res.Pmu_active_count = len(msrDel)
	}

	var js []byte
	js, err := json.Marshal(res)
	if err != nil {
		log.Errorf("Result could not be converted to json\nError: ", err)
		os.Exit(3)
	}
	fmt.Println(string(js))

}

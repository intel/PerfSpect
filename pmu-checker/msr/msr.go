package msr

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	msrPath           = "/dev/cpu/%d/msr"
	generalPurposePMU = "General_purpose_programmable_PMU"
)

var (
	Values     = sync.Map{}
	UsedPMUs   []string
	pmuPurpose = map[string]string{
		"0x309": "instructions",
		"0x30a": "cpu_cycles",
		"0x30b": "ref_cycles",
		"0xc1":  generalPurposePMU,
		"0xc2":  generalPurposePMU,
		"0xc3":  generalPurposePMU,
		"0xc4":  generalPurposePMU,
	}
)

type retMSR struct {
	fd int
}

func (dpt retMSR) read(msr int64) (uint64, error) {
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
		return nil, errors.New("Couldn't open the msr interface")
	}

	return &retMSR{fd: fd}, nil

}

func closeMSRInterface(dpt retMSR) error {
	// Close connection to MSR Interface
	return syscall.Close(dpt.fd)
}

func ReadMSR(reg string, wg *sync.WaitGroup, thread int, cpu int) {
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

	msrVal, err := msr.read(regInt64)
	if err != nil {
		log.Panic(err)
	}

	err = closeMSRInterface(*msr)
	if err != nil {
		log.Panic(err)
	}

	log.Debugf("New value of thread %d for %s is %d", thread, reg, msrVal)
	currentVal, found := Values.Load(reg)
	Values.Store(reg, msrVal)
	if found == false {
		// The key has been deleted, meaning PMU was active
	}

	log.Debugf("Old value of thread %d for %s is %d", thread, reg, currentVal)

	if found == true && currentVal != uint64(0) && msrVal != currentVal {
		// The key exists but value has changed, delete it

		UsedPMUs = append(UsedPMUs, reg)
		log.Debugf("Deleting %s in the thread %d", reg, thread)
		Values.Delete(reg)

	}
	log.Debugf("Worker %d done for %s\n", thread, reg)

}

func Initialize() error {
	for regPMU, _ := range pmuPurpose {
		Values.Store(regPMU, uint64(0))
	}

	return nil
}

func GetActivePMUs() (Result, error) {
	var res Result
	res.PMUDetails = make(map[string]string)
	log.Info("Following PMU(s) are actively being used:")
	for _, pmu := range UsedPMUs {
		purpose, ok := pmuPurpose[pmu]
		if !ok {
			return Result{}, errors.New("Report this to the Developers.")
		}
		res.PMUDetails[pmu] = purpose
		res.PMUActive++
		log.Infof("%s: might be using: %s", pmu, purpose)
	}

	return res, nil
}

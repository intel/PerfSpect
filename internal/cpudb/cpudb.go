/*
Package cpudb provides a reference of CPU architectures and identification keys for known CPUS.
*/
package cpudb

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type CPUDB []CPU

// NewCPUDB initializes the CPUDB structure with the yaml and returns it
func NewCPUDB() *CPUDB {
	return &cpus
}

// GetCPU retrieves the CPU structure that matches the provided args
func (c *CPUDB) GetCPU(family, model, stepping string) (cpu CPU, err error) {
	return c.GetCPUExtended(family, model, stepping, "", "")
}

// GetCPUExtended retrieves the CPU structure that matches the provided args
// capid4 needed to differentiate EMR MCC from EMR XCC
//
//	capid4: $ lspci -s $(lspci | grep 325b | awk 'NR==1{{print $1}}') -xxx |  awk '$1 ~ /^90/{{print $9 $8 $7 $6; exit}}'
//
// devices needed to differentiate GNR X1/2/3
//
//	devices: $ lspci -d 8086:3258 | wc -l
func (c *CPUDB) GetCPUExtended(family, model, stepping, capid4, devices string) (cpu CPU, err error) {
	for _, info := range *c {
		// if family matches
		if info.Family == family {
			var reModel *regexp.Regexp
			reModel, err = regexp.Compile(info.Model)
			if err != nil {
				return
			}
			// if model matches
			if reModel.FindString(model) == model {
				// if there is a stepping
				if info.Stepping != "" {
					var reStepping *regexp.Regexp
					reStepping, err = regexp.Compile(info.Stepping)
					if err != nil {
						return
					}
					// if stepping does NOT match
					if reStepping.FindString(stepping) == "" {
						// no match
						continue
					}
				}
				cpu = info
				if cpu.Family == "6" && (cpu.Model == "143" || cpu.Model == "207" || cpu.Model == "173") { // SPR, EMR, GNR
					cpu, err = c.getSpecificCPU(family, model, capid4, devices)
				}
				return
			}
		}
	}
	err = fmt.Errorf("CPU match not found for family %s, model %s, stepping %s", family, model, stepping)
	return
}

func (c *CPUDB) GetCPUByMicroArchitecture(uarch string) (cpu CPU, err error) {
	for _, info := range *c {
		if strings.EqualFold(info.MicroArchitecture, uarch) {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("CPU match not found for uarch %s", uarch)
	return
}

func (c *CPUDB) getSpecificCPU(family, model, capid4, devices string) (cpu CPU, err error) {
	if family == "6" && model == "143" { // SPR
		cpu, err = c.getSPRCPU(capid4)
	} else if family == "6" && model == "207" { // EMR
		cpu, err = c.getEMRCPU(capid4)
	} else if family == "6" && model == "173" { // GNR
		cpu, err = c.getGNRCPU(devices)
	}
	return
}

func (c *CPUDB) getSPRCPU(capid4 string) (cpu CPU, err error) {
	var uarch string
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		if bits == 3 {
			uarch = "SPR_XCC"
		} else if bits == 1 {
			uarch = "SPR_MCC"
		}
	}
	if uarch == "" {
		uarch = "SPR"
	}
	for _, info := range *c {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching SPR architecture in CPU database: %s", uarch)
	return
}

func (c *CPUDB) getEMRCPU(capid4 string) (cpu CPU, err error) {
	var uarch string
	if capid4 != "" {
		var bits int64
		var capid4Int int64
		capid4Int, err = strconv.ParseInt(capid4, 16, 64)
		if err != nil {
			return
		}
		bits = (capid4Int >> 6) & 0b11
		if bits == 3 {
			uarch = "EMR_XCC"
		} else if bits == 1 {
			uarch = "EMR_MCC"
		}
	}
	if uarch == "" {
		uarch = "EMR"
	}
	for _, info := range *c {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching EMR architecture in CPU database: %s", uarch)
	return
}

func (c *CPUDB) getGNRCPU(devices string) (cpu CPU, err error) {
	var uarch string
	if devices != "" {
		d, err := strconv.Atoi(devices)
		if err == nil && d != 0 {
			if d%5 == 0 { // device count is multiple of 5
				uarch = "GNR_X3"
			} else if d%4 == 0 { // device count is multiple of 4
				uarch = "GNR_X2"
			} else if d%3 == 0 { // device count is multiple of 3
				uarch = "GNR_X1"
			}
		}
	}
	if uarch == "" {
		uarch = "GNR"
	}
	for _, info := range *c {
		if info.MicroArchitecture == uarch {
			cpu = info
			return
		}
	}
	err = fmt.Errorf("did not find matching GNR architecture in CPU database: %s", uarch)
	return
}

func (cpu *CPU) GetCacheWays() (cacheWays []int64) {
	wayCount := cpu.CacheWayCount
	if wayCount == 0 {
		return
	}
	var cacheSize int64 = 0
	// set wayCount bits in cacheSize
	for i := 0; i < wayCount; i++ {
		cacheSize = (cacheSize << 1) | 1
	}
	var mask int64 = -1 // all bits set
	for i := 0; i < wayCount; i++ {
		// prepend the cache size to the list of ways
		cacheWays = append([]int64{cacheSize}, cacheWays...)
		// clear another low bit in mask
		mask = mask << 1
		// mask lower bits (however many bits are cleared in mask var)
		cacheSize = cacheSize & mask
	}
	return
}

package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log"
	"log/slog"
	"perfspect/internal/script"
	"regexp"
	"strconv"
	"strings"
)

const (
	BankLocatorIdx = iota
	LocatorIdx
	ManufacturerIdx
	PartIdx
	SerialIdx
	SizeIdx
	TypeIdx
	DetailIdx
	SpeedIdx
	RankIdx
	ConfiguredSpeedIdx
	DerivedSocketIdx
	DerivedChannelIdx
	DerivedSlotIdx
)

func dimmInfoFromDmiDecode(dmiDecodeOutput string) [][]string {
	return valsArrayFromDmiDecodeRegexSubmatch(
		dmiDecodeOutput,
		"17",
		`^Bank Locator:\s*(.+?)$`,
		`^Locator:\s*(.+?)$`,
		`^Manufacturer:\s*(.+?)$`,
		`^Part Number:\s*(.+?)\s*$`,
		`^Serial Number:\s*(.+?)\s*$`,
		`^Size:\s*(.+?)$`,
		`^Type:\s*(.+?)$`,
		`^Type Detail:\s*(.+?)$`,
		`^Speed:\s*(.+?)$`,
		`^Rank:\s*(.+?)$`,
		`^Configured.*Speed:\s*(.+?)$`,
	)
}

func installedMemoryFromOutput(outputs map[string]script.ScriptOutput) string {
	dimmInfo := dimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
	dimmTypeCount := make(map[string]int)
	for _, dimm := range dimmInfo {
		dimmKey := dimm[TypeIdx] + ":" + dimm[SizeIdx] + ":" + dimm[SpeedIdx] + ":" + dimm[ConfiguredSpeedIdx]
		if count, ok := dimmTypeCount[dimmKey]; ok {
			dimmTypeCount[dimmKey] = count + 1
		} else {
			dimmTypeCount[dimmKey] = 1
		}
	}
	var summaries []string
	re := regexp.MustCompile(`(\d+)\s*(\w*)`)
	for dimmKey, count := range dimmTypeCount {
		fields := strings.Split(dimmKey, ":")
		match := re.FindStringSubmatch(fields[1]) // size field
		if match != nil {
			size, err := strconv.Atoi(match[1])
			if err != nil {
				log.Printf("Don't recognize DIMM size format: %s", fields[1])
				return ""
			}
			sum := count * size
			unit := match[2]
			dimmType := fields[0]
			speed := strings.ReplaceAll(fields[2], " ", "")
			configuredSpeed := strings.ReplaceAll(fields[3], " ", "")
			summary := fmt.Sprintf("%d%s (%dx%d%s %s %s [%s])", sum, unit, count, size, unit, dimmType, speed, configuredSpeed)
			summaries = append(summaries, summary)
		}
	}
	return strings.Join(summaries, "; ")
}

func populatedChannelsFromOutput(outputs map[string]script.ScriptOutput) string {
	channelsMap := make(map[string]bool)
	dimmInfo := dimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
	derivedDimmFields := derivedDimmsFieldFromOutput(outputs)
	if len(derivedDimmFields) != len(dimmInfo) {
		slog.Error("derivedDimmFields and dimmInfo have different lengths", slog.Int("derivedDimmFields", len(derivedDimmFields)), slog.Int("dimmInfo", len(dimmInfo)))
		return ""
	}
	for i, dimm := range dimmInfo {
		if !strings.Contains(dimm[SizeIdx], "No") {
			channelsMap[derivedDimmFields[i].socket+","+derivedDimmFields[i].channel] = true
		}
	}
	if len(channelsMap) > 0 {
		return fmt.Sprintf("%d", len(channelsMap))
	}
	return ""
}

type derivedFields struct {
	socket  string
	channel string
	slot    string
}

// derivedDimmsFieldFromOutput returns a slice of derived fields from the output of a script.
func derivedDimmsFieldFromOutput(outputs map[string]script.ScriptOutput) []derivedFields {
	dimmInfo := dimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
	var derivedFields []derivedFields
	var err error
	channels := channelsFromOutput(outputs)
	numChannels, err := strconv.Atoi(channels)
	if err != nil || numChannels == 0 {
		return nil
	}
	platformVendor := valFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0", `Vendor:\s*(.*)`)
	numSockets, err := strconv.Atoi(valFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(.*:\s*(.+?)$`))
	if err != nil || numSockets == 0 {
		return nil
	}
	success := false
	if strings.Contains(platformVendor, "Dell") {
		derivedFields, err = deriveDIMMInfoDell(dimmInfo, numChannels)
		if err != nil {
			slog.Info("failed to parse dimm info on Dell platform", slog.String("error", err.Error()))
		}
		success = err == nil
	} else if platformVendor == "HPE" {
		derivedFields, err = deriveDIMMInfoHPE(dimmInfo, numSockets, numChannels)
		if err != nil {
			slog.Info("failed to parse dimm info on HPE platform", slog.String("error", err.Error()))
		}
		success = err == nil
	} else if platformVendor == "Amazon EC2" {
		derivedFields, err = deriveDIMMInfoEC2(dimmInfo, numChannels)
		if err != nil {
			slog.Info("failed to parse dimm info on Amazon EC2 platform", slog.String("error", err.Error()))
		}
		success = err == nil
	}
	if !success {
		derivedFields, err = deriveDIMMInfoOther(dimmInfo, numChannels)
		if err != nil {
			slog.Info("failed to parse dimm info on other platform", slog.String("error", err.Error()))
		}
	}
	return derivedFields
}

/* as seen on 2 socket Dell systems...
* "Bank Locator" for all DIMMs is "Not Specified" and "Locator" is A1-A12 and B1-B12.
* A1 and A7 are channel 0, A2 and A8 are channel 1, etc.
 */
func deriveDIMMInfoDell(dimms [][]string, channelsPerSocket int) ([]derivedFields, error) {
	derivedFields := make([]derivedFields, len(dimms))
	re := regexp.MustCompile(`([ABCD])([1-9]\d*)`)
	for i, dimm := range dimms {
		if !strings.Contains(dimm[BankLocatorIdx], "Not Specified") {
			err := fmt.Errorf("doesn't conform to expected Dell Bank Locator format")
			return nil, err
		}
		match := re.FindStringSubmatch(dimm[LocatorIdx])
		if match == nil {
			err := fmt.Errorf("doesn't conform to expected Dell Locator format")
			return nil, err
		}
		alpha := match[1]
		var numeric int
		numeric, err := strconv.Atoi(match[2])
		if err != nil {
			err = fmt.Errorf("doesn't conform to expected Dell Locator numeric format")
			return nil, err
		}
		// Socket
		// A = 0, B = 1, C = 2, D = 3
		derivedFields[i].socket = fmt.Sprintf("%d", int(alpha[0])-int('A'))
		// Slot
		if numeric <= channelsPerSocket {
			derivedFields[i].slot = "0"
		} else {
			derivedFields[i].slot = "1"
		}
		// Channel
		if numeric <= channelsPerSocket {
			derivedFields[i].channel = fmt.Sprintf("%d", numeric-1)
		} else {
			derivedFields[i].channel = fmt.Sprintf("%d", numeric-(channelsPerSocket+1))
		}
	}
	return derivedFields, nil
}

/* as seen on Amazon EC2 bare-metal systems...
 * 		BANK LOC		LOCATOR
 * c5.metal
 * 		NODE 1			DIMM_A0
 * 		NODE 1			DIMM_A1
 * 		...
 * 		NODE 2			DIMM_G0
 * 		NODE 2			DIMM_G1
 * 		...								<<< there's no 'I'
 * 		NODE 2			DIMM_M0
 * 		NODE 2			DIMM_M1
 *
 * c6i.metal
 * 		NODE 0			CPU0 Channel0 DIMM0
 * 		NODE 0			CPU0 Channel0 DIMM1
 * 		NODE 0			CPU0 Channel1 DIMM0
 * 		NODE 0			CPU0 Channel1 DIMM1
 * 		...
 * 		NODE 7			CPU1 Channel7 DIMM0
 * 		NODE 7			CPU1 Channel7 DIMM1
 */
func deriveDIMMInfoEC2(dimms [][]string, channelsPerSocket int) ([]derivedFields, error) {
	derivedFields := make([]derivedFields, len(dimms))
	c5bankLocRe := regexp.MustCompile(`NODE\s+([1-9])`)
	c5locRe := regexp.MustCompile(`DIMM_(.)(.)`)
	c6ibankLocRe := regexp.MustCompile(`NODE\s+(\d+)`)
	c6ilocRe := regexp.MustCompile(`CPU(\d+)\s+Channel(\d+)\s+DIMM(\d+)`)
	for i, dimm := range dimms {
		// try c5.metal format
		bankLocMatch := c5bankLocRe.FindStringSubmatch(dimm[BankLocatorIdx])
		locMatch := c5locRe.FindStringSubmatch(dimm[LocatorIdx])
		if locMatch != nil && bankLocMatch != nil {
			var socket, channel, slot int
			socket, _ = strconv.Atoi(bankLocMatch[1])
			socket -= 1
			if int(locMatch[1][0]) < int('I') { // there is no 'I'
				channel = (int(locMatch[1][0]) - int('A')) % channelsPerSocket
			} else if int(locMatch[1][0]) > int('I') {
				channel = (int(locMatch[1][0]) - int('B')) % channelsPerSocket
			} else {
				err := fmt.Errorf("doesn't conform to expected EC2 format")
				return nil, err
			}
			slot, _ = strconv.Atoi(locMatch[2])
			derivedFields[i].socket = fmt.Sprintf("%d", socket)
			derivedFields[i].channel = fmt.Sprintf("%d", channel)
			derivedFields[i].slot = fmt.Sprintf("%d", slot)
			continue
		}
		// try c6i.metal format
		bankLocMatch = c6ibankLocRe.FindStringSubmatch(dimm[BankLocatorIdx])
		locMatch = c6ilocRe.FindStringSubmatch(dimm[LocatorIdx])
		if locMatch != nil && bankLocMatch != nil {
			var socket, channel, slot int
			socket, _ = strconv.Atoi(locMatch[1])
			channel, _ = strconv.Atoi(locMatch[2])
			slot, _ = strconv.Atoi(locMatch[3])
			derivedFields[i].socket = fmt.Sprintf("%d", socket)
			derivedFields[i].channel = fmt.Sprintf("%d", channel)
			derivedFields[i].slot = fmt.Sprintf("%d", slot)
			continue
		}
		err := fmt.Errorf("doesn't conform to expected EC2 format")
		return nil, err
	}
	return derivedFields, nil
}

/* as seen on 2 socket HPE systems...2 slots per channel
* Locator field has these: PROC 1 DIMM 1, PROC 1 DIMM 2, etc...
* DIMM/slot numbering on board follows logic shown below
 */
func deriveDIMMInfoHPE(dimms [][]string, numSockets int, channelsPerSocket int) ([]derivedFields, error) {
	derivedFields := make([]derivedFields, len(dimms))
	slotsPerChannel := len(dimms) / (numSockets * channelsPerSocket)
	re := regexp.MustCompile(`PROC ([1-9]\d*) DIMM ([1-9]\d*)`)
	for i, dimm := range dimms {
		if !strings.Contains(dimm[BankLocatorIdx], "Not Specified") {
			err := fmt.Errorf("doesn't conform to expected HPE Bank Locator format: %s", dimm[BankLocatorIdx])
			return nil, err
		}
		match := re.FindStringSubmatch(dimm[LocatorIdx])
		if match == nil {
			err := fmt.Errorf("doesn't conform to expected HPE Locator format: %s", dimm[LocatorIdx])
			return nil, err
		}
		socket, err := strconv.Atoi(match[1])
		if err != nil {
			err := fmt.Errorf("failed to parse socket number: %s", match[1])
			return nil, err
		}
		socket -= 1
		derivedFields[i].socket = fmt.Sprintf("%d", socket)
		dimmNum, err := strconv.Atoi(match[2])
		if err != nil {
			err := fmt.Errorf("failed to parse DIMM number: %s", match[2])
			return nil, err
		}
		channel := (dimmNum - 1) / slotsPerChannel
		derivedFields[i].channel = fmt.Sprintf("%d", channel)
		var slot int
		if (dimmNum < channelsPerSocket && dimmNum%2 != 0) || (dimmNum > channelsPerSocket && dimmNum%2 == 0) {
			slot = 0
		} else {
			slot = 1
		}
		derivedFields[i].slot = fmt.Sprintf("%d", slot)
	}
	return derivedFields, nil
}

/*
Get DIMM socket and slot from Bank Locator or Locator field from dmidecode.
This method is inherently unreliable/incomplete as each OEM can set
these fields as they see fit.
Returns None when there's no match.
*/
func getDIMMSocketSlot(dimmType DIMMType, reBankLoc *regexp.Regexp, reLoc *regexp.Regexp, bankLocator string, locator string) (socket int, slot int, err error) {
	if dimmType == DIMMType0 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
		}
		return
	} else if dimmType == DIMMType1 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			return
		}
	} else if dimmType == DIMMType2 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			return
		}
	} else if dimmType == DIMMType3 {
		match := reBankLoc.FindStringSubmatch(bankLocator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			return
		}
	} else if dimmType == DIMMType4 {
		match := reBankLoc.FindStringSubmatch(bankLocator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[4])
			return
		}
	} else if dimmType == DIMMType5 {
		match := reBankLoc.FindStringSubmatch(bankLocator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			return
		}
	} else if dimmType == DIMMType6 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			socket -= 1
			slot, _ = strconv.Atoi(match[3])
			slot -= 1
			return
		}
	} else if dimmType == DIMMType7 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			slot -= 1
			return
		}
	} else if dimmType == DIMMType8 {
		match := reBankLoc.FindStringSubmatch(bankLocator)
		if match != nil {
			match2 := reLoc.FindStringSubmatch(locator)
			if match2 != nil {
				socket, _ = strconv.Atoi(match[1])
				socket -= 1
				slot, _ = strconv.Atoi(match2[2])
				slot -= 1
				return
			}
		}
	} else if dimmType == DIMMType9 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[2])
			return
		}
	} else if dimmType == DIMMType10 {
		match := reBankLoc.FindStringSubmatch(bankLocator)
		if match != nil {
			socket = 0
			slot, _ = strconv.Atoi(match[2])
			return
		}
	} else if dimmType == DIMMType11 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket = 0
			slot, _ = strconv.Atoi(match[2])
			return
		}
	} else if dimmType == DIMMType12 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			socket = socket - 1
			slot, _ = strconv.Atoi(match[3])
			slot = slot - 1
			return
		}
	} else if dimmType == DIMMType13 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			slot = slot - 1
			return
		}
	} else if dimmType == DIMMType14 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot = 0
			return
		}
	} else if dimmType == DIMMType15 {
		match := reLoc.FindStringSubmatch(locator)
		if match != nil {
			socket, _ = strconv.Atoi(match[1])
			slot, _ = strconv.Atoi(match[3])
			return
		}
	}
	err = fmt.Errorf("unrecognized bank locator and/or locator in dimm info: %s %s", bankLocator, locator)
	return
}

type DIMMType int

const (
	DIMMTypeUNKNOWN          = -1
	DIMMType0       DIMMType = iota
	DIMMType1
	DIMMType2
	DIMMType3
	DIMMType4
	DIMMType5
	DIMMType6
	DIMMType7
	DIMMType8
	DIMMType9
	DIMMType10
	DIMMType11
	DIMMType12
	DIMMType13
	DIMMType14
	DIMMType15
)

func getDIMMParseInfo(bankLocator string, locator string) (dimmType DIMMType, reBankLoc *regexp.Regexp, reLoc *regexp.Regexp) {
	dimmType = DIMMTypeUNKNOWN
	// Inspur ICX 2s system
	// Needs to be before next regex pattern to differentiate
	reLoc = regexp.MustCompile(`CPU([0-9])_C([0-9])D([0-9])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType0
		return
	}
	reLoc = regexp.MustCompile(`CPU([0-9])_([A-Z])([0-9])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType1
		return
	}
	reLoc = regexp.MustCompile(`CPU([0-9])_MC._DIMM_([A-Z])([0-9])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType2
		return
	}
	reBankLoc = regexp.MustCompile(`NODE ([0-9]) CHANNEL ([0-9]) DIMM ([0-9])`)
	if reBankLoc.FindStringSubmatch(bankLocator) != nil {
		dimmType = DIMMType3
		return
	}
	/* Added for SuperMicro X13DET-B (SPR). Must be before Type4 because Type4 matches, but data in BankLoc is invalid.
	 * Locator: P1-DIMMA1
	 * Locator: P1-DIMMB1
	 * Locator: P1-DIMMC1
	 * ...
	 * Locator: P2-DIMMA1
	 * ...
	 * Note: also matches SuperMicro X11DPT-B (CLX)
	 */
	reLoc = regexp.MustCompile(`P([1,2])-DIMM([A-L])([1,2])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType12
		return
	}
	reBankLoc = regexp.MustCompile(`P([0-9])_Node([0-9])_Channel([0-9])_Dimm([0-9])`)
	if reBankLoc.FindStringSubmatch(bankLocator) != nil {
		dimmType = DIMMType4
		return
	}
	reBankLoc = regexp.MustCompile(`_Node([0-9])_Channel([0-9])_Dimm([0-9])`)
	if reBankLoc.FindStringSubmatch(bankLocator) != nil {
		dimmType = DIMMType5
		return
	}
	/* SKX SDP
	 * Locator: CPU1_DIMM_A1, Bank Locator: NODE 1
	 * Locator: CPU1_DIMM_A2, Bank Locator: NODE 1
	 */
	reLoc = regexp.MustCompile(`CPU([1-4])_DIMM_([A-Z])([1-2])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		reBankLoc = regexp.MustCompile(`NODE ([1-8])`)
		if reBankLoc.FindStringSubmatch(bankLocator) != nil {
			dimmType = DIMMType6
			return
		}
	}
	/* ICX SDP
	 * Locator: CPU0_DIMM_A1, Bank Locator: NODE 0
	 * Locator: CPU0_DIMM_A2, Bank Locator: NODE 0
	 */
	reLoc = regexp.MustCompile(`CPU([0-7])_DIMM_([A-Z])([1-2])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		reBankLoc = regexp.MustCompile(`NODE ([0-9]+)`)
		if reBankLoc.FindStringSubmatch(bankLocator) != nil {
			dimmType = DIMMType7
			return
		}
	}
	reBankLoc = regexp.MustCompile(`NODE ([1-9]\d*)`)
	if reBankLoc.FindStringSubmatch(bankLocator) != nil {
		reLoc = regexp.MustCompile(`DIMM_([A-Z])([1-9]\d*)`)
		if reLoc.FindStringSubmatch(locator) != nil {
			dimmType = DIMMType8
			return
		}
	}
	/* GIGABYTE MILAN
	 * Locator: DIMM_P0_A0, Bank Locator: BANK 0
	 * Locator: DIMM_P0_A1, Bank Locator: BANK 1
	 * Locator: DIMM_P0_B0, Bank Locator: BANK 0
	 * ...
	 * Locator: DIMM_P1_I0, Bank Locator: BANK 0
	 */
	reLoc = regexp.MustCompile(`DIMM_P([0-1])_[A-Z]([0-1])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType9
		return
	}
	/* my NUC
	 * Locator: SODIMM0, Bank Locator: CHANNEL A DIMM0
	 * Locator: SODIMM1, Bank Locator: CHANNEL B DIMM0
	 */
	reBankLoc = regexp.MustCompile(`CHANNEL ([A-D]) DIMM([0-9])`)
	if reBankLoc.FindStringSubmatch(bankLocator) != nil {
		dimmType = DIMMType10
		return
	}
	/* Alder Lake Client Desktop
	 * Locator: Controller0-ChannelA-DIMM0, Bank Locator: BANK 0
	 * Locator: Controller1-ChannelA-DIMM0, Bank Locator: BANK 0
	 */
	reLoc = regexp.MustCompile(`Controller([0-1]).*DIMM([0-1])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType11
		return
	}
	/* BIRCHSTREAM
	 * LOCATOR      BANK LOCATOR
	 * CPU0_DIMM_A1 BANK 0
	 * CPU0_DIMM_A2 BANK 0
	 * CPU0_DIMM_B1 BANK 1
	 * CPU0_DIMM_B2 BANK 1
	 * ...
	 * CPU0_DIMM_H2 BANK 7
	 */
	reLoc = regexp.MustCompile(`CPU([\d])_DIMM_([A-H])([1-2])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType13
		return
	}
	/* BIRCHSTREAM GRANITE RAPIDS AP/X3
	 * LOCATOR      BANK LOCATOR
	 * CPU0_DIMM_A  BANK 0
	 * CPU0_DIMM_B  BANK 1
	 * CPU0_DIMM_C  BANK 2
	 * CPU0_DIMM_D  BANK 3
	 * ...
	 * CPU0_DIMM_L  BANK 11
	 */
	reLoc = regexp.MustCompile(`CPU([\d])_DIMM_([A-L])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType14
		return
	}
	/* FOREST CITY PLATFORM FOR SRF AND GNR
	 * LOCATOR      BANK LOCATOR
	 * CPU0 CH0/D0  BANK 0
	 * CPU0 CH0/D1  BANK 0
	 * CPU0 CH1/D0  BANK 1
	 * CPU0 CH1/D1  BANK 1
	 * ...
	 * CPU0 CH7/D1  BANK 7
	 */
	reLoc = regexp.MustCompile(`CPU([\d]) CH([0-7])/D([0-1])`)
	if reLoc.FindStringSubmatch(locator) != nil {
		dimmType = DIMMType15
		return
	}
	return
}

func deriveDIMMInfoOther(dimms [][]string, channelsPerSocket int) ([]derivedFields, error) {
	derivedFields := make([]derivedFields, len(dimms))
	previousSocket, channel := -1, 0
	if len(dimms) == 0 {
		err := fmt.Errorf("no DIMMs")
		return nil, err
	}
	dimmType, reBankLoc, reLoc := getDIMMParseInfo((dimms)[0][BankLocatorIdx], (dimms)[0][LocatorIdx])
	if dimmType == DIMMTypeUNKNOWN {
		err := fmt.Errorf("unknown DIMM identification format")
		return nil, err
	}
	for i, dimm := range dimms {
		var socket, slot int
		socket, slot, err := getDIMMSocketSlot(dimmType, reBankLoc, reLoc, dimm[BankLocatorIdx], dimm[LocatorIdx])
		if err != nil {
			slog.Info("Couldn't extract socket and slot from DIMM info", slog.String("error", err.Error()))
			return nil, nil
		}
		if socket > previousSocket {
			channel = 0
		} else if previousSocket == socket && slot == 0 {
			channel++
		}
		// sanity check
		if channel >= channelsPerSocket {
			err := fmt.Errorf("invalid interpretation of DIMM data")
			return nil, err
		}
		previousSocket = socket
		derivedFields[i].socket = fmt.Sprintf("%d", socket)
		derivedFields[i].channel = fmt.Sprintf("%d", channel)
		derivedFields[i].slot = fmt.Sprintf("%d", slot)
	}
	return derivedFields, nil
}

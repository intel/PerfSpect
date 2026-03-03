// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"perfspect/internal/script"
)

// DerivedDIMMFields holds derived socket, channel, and slot information for a DIMM.
type DerivedDIMMFields struct {
	Socket  string
	Channel string
	Slot    string
}

// DIMM info field indices
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

// InstalledMemoryFromOutput returns a summary of installed memory from script outputs.
func InstalledMemoryFromOutput(outputs map[string]script.ScriptOutput) string {
	dimmInfo := DimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
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
				slog.Warn("Don't recognize DIMM size format.", slog.String("field", fields[1]))
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

// PopulatedChannelsFromOutput returns the number of populated memory channels.
func PopulatedChannelsFromOutput(outputs map[string]script.ScriptOutput) string {
	channelsMap := make(map[string]bool)
	dimmInfo := DimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
	derivedDimmFields := DerivedDimmsFieldFromOutput(outputs)
	if len(derivedDimmFields) != len(dimmInfo) {
		slog.Warn("derivedDimmFields and dimmInfo have different lengths", slog.Int("derivedDimmFields", len(derivedDimmFields)), slog.Int("dimmInfo", len(dimmInfo)))
		return ""
	}
	for i, dimm := range dimmInfo {
		if !strings.Contains(dimm[SizeIdx], "No") {
			channelsMap[derivedDimmFields[i].Socket+","+derivedDimmFields[i].Channel] = true
		}
	}
	if len(channelsMap) > 0 {
		return fmt.Sprintf("%d", len(channelsMap))
	}
	return ""
}

// DerivedDimmsFieldFromOutput returns a slice of derived fields from the output of a script.
func DerivedDimmsFieldFromOutput(outputs map[string]script.ScriptOutput) []DerivedDIMMFields {
	dimmInfo := DimmInfoFromDmiDecode(outputs[script.DmidecodeScriptName].Stdout)
	var derivedFields []DerivedDIMMFields
	var err error
	channels := ChannelsFromOutput(outputs)
	numChannels, err := strconv.Atoi(channels)
	if err != nil || numChannels == 0 {
		return nil
	}
	platformVendor := ValFromDmiDecodeRegexSubmatch(outputs[script.DmidecodeScriptName].Stdout, "0", `Vendor:\s*(.*)`)
	numSockets, err := strconv.Atoi(ValFromRegexSubmatch(outputs[script.LscpuScriptName].Stdout, `^Socket\(.*:\s*(.+?)$`))
	if err != nil || numSockets == 0 {
		return nil
	}
	success := false
	if strings.Contains(platformVendor, "Dell") {
		derivedFields, err = deriveDIMMInfoDell(dimmInfo, numChannels)
		if err != nil {
			slog.Warn("failed to parse dimm info on Dell platform", slog.String("error", err.Error()))
		}
		success = err == nil
	} else if platformVendor == "HPE" {
		derivedFields, err = deriveDIMMInfoHPE(dimmInfo, numSockets, numChannels)
		if err != nil {
			slog.Warn("failed to parse dimm info on HPE platform", slog.String("error", err.Error()))
		}
		success = err == nil
	} else if platformVendor == "Amazon EC2" {
		derivedFields, err = deriveDIMMInfoEC2(dimmInfo, numChannels)
		if err != nil {
			slog.Warn("failed to parse dimm info on Amazon EC2 platform", slog.String("error", err.Error()))
		}
		success = err == nil
	}
	if !success {
		derivedFields, err = deriveDIMMInfoOther(dimmInfo, numChannels)
		if err != nil {
			slog.Warn("failed to parse dimm info on other platform", slog.String("error", err.Error()))
		}
	}
	return derivedFields
}

/* as seen on 2 socket Dell systems...
* "Bank Locator" for all DIMMs is "Not Specified" and "Locator" is A1-A12 and B1-B12.
* A1 and A7 are channel 0, A2 and A8 are channel 1, etc.
 */
func deriveDIMMInfoDell(dimms [][]string, channelsPerSocket int) ([]DerivedDIMMFields, error) {
	derivedFields := make([]DerivedDIMMFields, len(dimms))
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
		derivedFields[i].Socket = fmt.Sprintf("%d", int(alpha[0])-int('A'))
		// Slot
		if numeric <= channelsPerSocket {
			derivedFields[i].Slot = "0"
		} else {
			derivedFields[i].Slot = "1"
		}
		// Channel
		if numeric <= channelsPerSocket {
			derivedFields[i].Channel = fmt.Sprintf("%d", numeric-1)
		} else {
			derivedFields[i].Channel = fmt.Sprintf("%d", numeric-(channelsPerSocket+1))
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
func deriveDIMMInfoEC2(dimms [][]string, channelsPerSocket int) ([]DerivedDIMMFields, error) {
	derivedFields := make([]DerivedDIMMFields, len(dimms))
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
			derivedFields[i].Socket = fmt.Sprintf("%d", socket)
			derivedFields[i].Channel = fmt.Sprintf("%d", channel)
			derivedFields[i].Slot = fmt.Sprintf("%d", slot)
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
			derivedFields[i].Socket = fmt.Sprintf("%d", socket)
			derivedFields[i].Channel = fmt.Sprintf("%d", channel)
			derivedFields[i].Slot = fmt.Sprintf("%d", slot)
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
func deriveDIMMInfoHPE(dimms [][]string, numSockets int, channelsPerSocket int) ([]DerivedDIMMFields, error) {
	derivedFields := make([]DerivedDIMMFields, len(dimms))
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
		derivedFields[i].Socket = fmt.Sprintf("%d", socket)
		dimmNum, err := strconv.Atoi(match[2])
		if err != nil {
			err := fmt.Errorf("failed to parse DIMM number: %s", match[2])
			return nil, err
		}
		channel := (dimmNum - 1) / slotsPerChannel
		derivedFields[i].Channel = fmt.Sprintf("%d", channel)
		var slot int
		if (dimmNum < channelsPerSocket && dimmNum%2 != 0) || (dimmNum > channelsPerSocket && dimmNum%2 == 0) {
			slot = 0
		} else {
			slot = 1
		}
		derivedFields[i].Slot = fmt.Sprintf("%d", slot)
	}
	return derivedFields, nil
}

type dimmType int

const (
	dimmTypeUNKNOWN dimmType = iota
	dimmTypeInspurICX
	dimmTypeQuantaGNR
	dimmTypeGenericCPULetterDigit
	dimmTypeMCFormat
	dimmTypeNodeChannelDimm
	dimmTypeSuperMicroSPR
	dimmTypePNodeChannelDimm
	dimmTypeNodeChannelDimmAlt
	dimmTypeSKXSDP
	dimmTypeICXSDP
	dimmTypeNodeDIMM
	dimmTypeGigabyteMilan
	dimmTypeNUC
	dimmTypeAlderLake
	dimmTypeBirchstream
	dimmTypeBirchstreamGNRAP
	dimmTypeForestCity
)

// dimmFormat defines how to identify and extract socket/slot for a DIMM format.
type dimmFormat struct {
	name       string
	dType      dimmType
	bankLocPat *regexp.Regexp // nil if bank locator not used for matching
	locPat     *regexp.Regexp // nil if locator not used for matching
	matchBoth  bool           // true = both patterns must match
	// extractFunc extracts socket and slot from regex matches.
	// bankLocMatch/locMatch are nil when the corresponding pattern is nil or didn't match.
	extractFunc func(bankLocMatch, locMatch []string) (socket, slot int, err error)
}

// dimmFormats is the ordered list of DIMM format definitions. Order matters:
// more specific patterns must appear before more general ones.
var dimmFormats = []dimmFormat{
	{
		// Inspur ICX 2s system — must be before GenericCPULetterDigit to differentiate
		name:   "Inspur ICX",
		dType:  dimmTypeInspurICX,
		locPat: regexp.MustCompile(`CPU([0-9])_C([0-9])D([0-9])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			return
		},
	},
	{
		// Quanta GNR — must match BOTH bank locator and locator.
		// Explicitly excludes Dimm0; must be before NodeChannelDimmAlt (type5).
		name:       "Quanta GNR",
		dType:      dimmTypeQuantaGNR,
		bankLocPat: regexp.MustCompile(`_Node(\d+)_Channel(\d+)_Dimm([1-2])\b`),
		locPat:     regexp.MustCompile(`CPU(\d+)_([A-Z])([1-2])\b`),
		matchBoth:  true,
		extractFunc: func(bankLocMatch, _ []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(bankLocMatch[1])
			slot, _ = strconv.Atoi(bankLocMatch[3])
			slot -= 1
			return
		},
	},
	{
		name:   "Generic CPU_Letter_Digit",
		dType:  dimmTypeGenericCPULetterDigit,
		locPat: regexp.MustCompile(`CPU([0-9])_([A-Z])([0-9])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			return
		},
	},
	{
		name:   "MC Format",
		dType:  dimmTypeMCFormat,
		locPat: regexp.MustCompile(`CPU([0-9])_MC._DIMM_([A-Z])([0-9])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			return
		},
	},
	{
		name:       "NODE CHANNEL DIMM",
		dType:      dimmTypeNodeChannelDimm,
		bankLocPat: regexp.MustCompile(`NODE ([0-9]) CHANNEL ([0-9]) DIMM ([0-9])`),
		extractFunc: func(bankLocMatch, _ []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(bankLocMatch[1])
			slot, _ = strconv.Atoi(bankLocMatch[3])
			return
		},
	},
	{
		// SuperMicro X13DET-B (SPR) / X11DPT-B (CLX).
		// Must be before PNodeChannelDimm because that pattern also matches, but bank loc data is invalid.
		name:   "SuperMicro SPR",
		dType:  dimmTypeSuperMicroSPR,
		locPat: regexp.MustCompile(`P([1,2])-DIMM([A-L])([1,2])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			socket -= 1
			slot, _ = strconv.Atoi(locMatch[3])
			slot -= 1
			return
		},
	},
	{
		name:       "P_Node_Channel_Dimm",
		dType:      dimmTypePNodeChannelDimm,
		bankLocPat: regexp.MustCompile(`P([0-9])_Node([0-9])_Channel([0-9])_Dimm([0-9])`),
		extractFunc: func(bankLocMatch, _ []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(bankLocMatch[1])
			slot, _ = strconv.Atoi(bankLocMatch[4])
			return
		},
	},
	{
		name:       "_Node_Channel_Dimm",
		dType:      dimmTypeNodeChannelDimmAlt,
		bankLocPat: regexp.MustCompile(`_Node([0-9])_Channel([0-9])_Dimm([0-9])`),
		extractFunc: func(bankLocMatch, _ []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(bankLocMatch[1])
			slot, _ = strconv.Atoi(bankLocMatch[3])
			return
		},
	},
	{
		// SKX SDP: CPU[1-4]_DIMM_[A-Z][1-2] with NODE [1-8]
		name:       "SKX SDP",
		dType:      dimmTypeSKXSDP,
		locPat:     regexp.MustCompile(`CPU([1-4])_DIMM_([A-Z])([1-2])`),
		bankLocPat: regexp.MustCompile(`NODE ([1-8])`),
		matchBoth:  true,
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			socket -= 1
			slot, _ = strconv.Atoi(locMatch[3])
			slot -= 1
			return
		},
	},
	{
		// ICX SDP: CPU[0-7]_DIMM_[A-Z][1-2] with NODE [0-9]+
		name:       "ICX SDP",
		dType:      dimmTypeICXSDP,
		locPat:     regexp.MustCompile(`CPU([0-7])_DIMM_([A-Z])([1-2])`),
		bankLocPat: regexp.MustCompile(`NODE ([0-9]+)`),
		matchBoth:  true,
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			slot -= 1
			return
		},
	},
	{
		// NODE n + DIMM_Xn (both must match)
		name:       "NODE DIMM",
		dType:      dimmTypeNodeDIMM,
		bankLocPat: regexp.MustCompile(`NODE ([1-9]\d*)`),
		locPat:     regexp.MustCompile(`DIMM_([A-Z])([1-9]\d*)`),
		matchBoth:  true,
		extractFunc: func(bankLocMatch, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(bankLocMatch[1])
			socket -= 1
			slot, _ = strconv.Atoi(locMatch[2])
			slot -= 1
			return
		},
	},
	{
		// Gigabyte Milan: DIMM_P[0-1]_[A-Z][0-1]
		name:   "Gigabyte Milan",
		dType:  dimmTypeGigabyteMilan,
		locPat: regexp.MustCompile(`DIMM_P([0-1])_[A-Z]([0-1])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[2])
			return
		},
	},
	{
		// NUC: CHANNEL [A-D] DIMM[0-9]
		name:       "NUC SODIMM",
		dType:      dimmTypeNUC,
		bankLocPat: regexp.MustCompile(`CHANNEL ([A-D]) DIMM([0-9])`),
		extractFunc: func(bankLocMatch, _ []string) (socket, slot int, err error) {
			socket = 0
			slot, _ = strconv.Atoi(bankLocMatch[2])
			return
		},
	},
	{
		// Alder Lake Client Desktop: Controller[0-1]-Channel*-DIMM[0-1]
		name:   "Alder Lake",
		dType:  dimmTypeAlderLake,
		locPat: regexp.MustCompile(`Controller([0-1]).*DIMM([0-1])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket = 0
			slot, _ = strconv.Atoi(locMatch[2])
			return
		},
	},
	{
		// Birchstream: CPU[0-9]_DIMM_[A-H][1-2]
		name:   "Birchstream",
		dType:  dimmTypeBirchstream,
		locPat: regexp.MustCompile(`CPU(\d)_DIMM_([A-H])([1-2])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			slot -= 1
			return
		},
	},
	{
		// Birchstream Granite Rapids AP/X3: CPU[0-9]_DIMM_[A-L] (no slot digit)
		name:   "Birchstream GNR AP/X3",
		dType:  dimmTypeBirchstreamGNRAP,
		locPat: regexp.MustCompile(`CPU(\d)_DIMM_([A-L])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot = 0
			return
		},
	},
	{
		// Forest City platform for SRF and GNR: CPU[0-9] CH[0-7]/D[0-1]
		name:   "Forest City SRF/GNR",
		dType:  dimmTypeForestCity,
		locPat: regexp.MustCompile(`CPU(\d) CH([0-7])/D([0-1])`),
		extractFunc: func(_, locMatch []string) (socket, slot int, err error) {
			socket, _ = strconv.Atoi(locMatch[1])
			slot, _ = strconv.Atoi(locMatch[3])
			return
		},
	},
}

// getDIMMParseInfo identifies the DIMM format from a representative bank locator and locator string.
func getDIMMParseInfo(bankLocator string, locator string) (dt dimmType, reBankLoc *regexp.Regexp, reLoc *regexp.Regexp) {
	for _, f := range dimmFormats {
		bankLocOK := f.bankLocPat == nil || f.bankLocPat.FindStringSubmatch(bankLocator) != nil
		locOK := f.locPat == nil || f.locPat.FindStringSubmatch(locator) != nil
		if f.matchBoth {
			if bankLocOK && locOK {
				return f.dType, f.bankLocPat, f.locPat
			}
		} else if f.bankLocPat != nil && bankLocOK {
			return f.dType, f.bankLocPat, f.locPat
		} else if f.locPat != nil && locOK {
			return f.dType, f.bankLocPat, f.locPat
		}
	}
	return dimmTypeUNKNOWN, nil, nil
}

// getDIMMSocketSlot extracts socket and slot from bank locator and locator strings
// using the format identified by getDIMMParseInfo.
func getDIMMSocketSlot(dt dimmType, reBankLoc *regexp.Regexp, reLoc *regexp.Regexp, bankLocator string, locator string) (socket int, slot int, err error) {
	for _, f := range dimmFormats {
		if f.dType != dt {
			continue
		}
		var bankLocMatch, locMatch []string
		if f.bankLocPat != nil {
			bankLocMatch = reBankLoc.FindStringSubmatch(bankLocator)
		}
		if f.locPat != nil {
			locMatch = reLoc.FindStringSubmatch(locator)
		}
		if bankLocMatch != nil || locMatch != nil {
			return f.extractFunc(bankLocMatch, locMatch)
		}
		break
	}
	err = fmt.Errorf("unrecognized bank locator and/or locator in dimm info: %s %s", bankLocator, locator)
	return
}

func deriveDIMMInfoOther(dimms [][]string, channelsPerSocket int) ([]DerivedDIMMFields, error) {
	derivedFields := make([]DerivedDIMMFields, len(dimms))
	previousSocket, channel := -1, 0
	if len(dimms) == 0 {
		err := fmt.Errorf("no DIMMs")
		return nil, err
	}
	if len(dimms[0]) <= max(BankLocatorIdx, LocatorIdx) {
		err := fmt.Errorf("DIMM data has insufficient fields")
		return nil, err
	}
	dt, reBankLoc, reLoc := getDIMMParseInfo(dimms[0][BankLocatorIdx], dimms[0][LocatorIdx])
	if dt == dimmTypeUNKNOWN {
		err := fmt.Errorf("unknown DIMM identification format")
		return nil, err
	}
	for i, dimm := range dimms {
		var socket, slot int
		socket, slot, err := getDIMMSocketSlot(dt, reBankLoc, reLoc, dimm[BankLocatorIdx], dimm[LocatorIdx])
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
		derivedFields[i].Socket = fmt.Sprintf("%d", socket)
		derivedFields[i].Channel = fmt.Sprintf("%d", channel)
		derivedFields[i].Slot = fmt.Sprintf("%d", slot)
	}
	return derivedFields, nil
}

// DimmInfoFromDmiDecode extracts DIMM information from DMI decode output.
func DimmInfoFromDmiDecode(dmiDecodeOutput string) [][]string {
	return ValsArrayFromDmiDecodeRegexSubmatch(
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

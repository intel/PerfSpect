// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"perfspect/internal/script"
)

// ISA represents an instruction set architecture extension.
type ISA struct {
	Name     string
	FullName string
	CPUID    string
}

// ISADefinitions contains all known ISA extension definitions.
var ISADefinitions = []ISA{
	{"AES", "Advanced Encryption Standard New Instructions (AES-NI)", "AES instruction"},
	{"AMX", "Advanced Matrix Extensions (AMX)", "AMX-BF16: tile bfloat16 support"},
	{"AMX-COMPLEX", "AMX-COMPLEX Instruction", "AMX-COMPLEX instructions"},
	{"AMX-FP16", "AMX-FP16 Instruction", "AMX-FP16: FP16 tile operations"},
	{"AVX-IFMA", "AVX-IFMA Instruction", "AVX-IFMA: integer fused multiply add"},
	{"AVX-NE-CONVERT", "AVX-NE-CONVERT Instruction", "AVX-NE-CONVERT instructions"},
	{"AVX-VNNI-INT8", "AVX-VNNI-INT8 Instruction", "AVX-VNNI-INT8 instructions"},
	{"AVX512F", "AVX-512 Foundation", "AVX512F: AVX-512 foundation instructions"},
	{"AVX512_BF16", "Vector Neural Network Instructions (AVX512_BF16)", "AVX512_BF16: bfloat16 instructions"},
	{"AVX512_FP16", "Advanced Vector Extensions (AVX512_FP16)", "AVX512_FP16: fp16 support"},
	{"AVX512_VNNI", "Vector Neural Network Instructions (AVX512_VNNI)", "AVX512_VNNI: neural network instructions"},
	{"CLDEMOTE", "Cache Line Demote (CLDEMOTE)", "CLDEMOTE supports cache line demote"},
	{"CMPCCXADD", "Compare and Add if Condition is Met (CMPCCXADD)", "CMPccXADD instructions"},
	{"ENQCMD", "Enqueue Command Instruction (ENQCMD)", "ENQCMD instruction"},
	{"MOVDIRI", "Move Doubleword as Direct Store (MOVDIRI)", "MOVDIRI instruction"},
	{"MOVDIR64B", "Move 64 Bytes as Direct Store (MOVDIR64B)", "MOVDIR64B instruction"},
	{"PREFETCHIT0/1", "PREFETCHIT0/1 Instruction", "PREFETCHIT0, PREFETCHIT1 instructions"},
	{"SERIALIZE", "SERIALIZE Instruction", "SERIALIZE instruction"},
	{"SHA_NI", "SHA1/SHA256 Instruction Extensions (SHA_NI)", "SHA instructions"},
	{"TSXLDTRK", "Transactional Synchronization Extensions (TSXLDTRK)", "TSXLDTRK: TSX suspend load addr tracking"},
	{"VAES", "Vector AES", "VAES instructions"},
	{"WAITPKG", "UMONITOR, UMWAIT, TPAUSE Instructions", "WAITPKG instructions"},
}

// ISAFullNames returns the full names of all ISA extensions.
func ISAFullNames() []string {
	var names []string
	for _, isa := range ISADefinitions {
		names = append(names, isa.FullName)
	}
	return names
}

// YesIfTrue converts a boolean string to "Yes" or "No".
func YesIfTrue(val string) string {
	if val == "true" {
		return "Yes"
	}
	return "No"
}

// ISASupportedFromOutput returns ISA support status from cpuid output.
func ISASupportedFromOutput(outputs map[string]script.ScriptOutput) []string {
	var supported []string
	for _, isa := range ISADefinitions {
		oneSupported := YesIfTrue(ValFromRegexSubmatch(outputs[script.CpuidScriptName].Stdout, isa.CPUID+`\s*= (.+?)$`))
		supported = append(supported, oneSupported)
	}
	return supported
}

package extract

import "perfspect/internal/script"

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

// OperatingSystemFromOutput returns the operating system from script outputs.
func OperatingSystemFromOutput(outputs map[string]script.ScriptOutput) string {
	os := ValFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^PRETTY_NAME=\"(.+?)\"`)
	centos := ValFromRegexSubmatch(outputs[script.EtcReleaseScriptName].Stdout, `^(CentOS Linux release .*)`)
	if centos != "" {
		os = centos
	}
	return os
}

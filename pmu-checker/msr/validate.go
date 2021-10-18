//###########################################################################################################
//# Copyright (C) 2021 Intel Corporation
//# SPDX-License-Identifier: BSD-3-Clause
//###########################################################################################################

package msr

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

func validate(path string) error {
	if _, err := os.Stat(path); err != nil {
		return errors.Wrap(err, fmt.Sprintf("MSR modules aren't loaded at %s, please load them using modprobe msr command", path))
	}
	return nil
}

func ValidateMSRModule(cpu int) error {
	msrDir := fmt.Sprintf(msrPath, cpu)
	return validate(msrDir)
}

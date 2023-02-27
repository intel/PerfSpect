//###########################################################################################################
//# Copyright (C) 2021 Intel Corporation
//# SPDX-License-Identifier: BSD-3-Clause
//###########################################################################################################

package msr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	msrExists := "./testdata/dev/cpu/1/msr"
	err := validate(msrExists)
	require.NoError(t, err)

	noExistingMSR := "./testdata/dev/cpu/2/msr"
	err = validate(noExistingMSR)
	require.EqualError(t, err, "MSR modules aren't loaded at ./testdata/dev/cpu/2/msr, please load them using modprobe msr command: stat ./testdata/dev/cpu/2/msr: no such file or directory")
}

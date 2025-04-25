package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import "testing"

func TestTransformConditional(t *testing.T) {
	var in string
	var out string
	var err error

	in = "100 * x / y"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != in {
		t.Error("out should equal in")
	}

	in = "100 * x / y if z"
	if _, err = transformConditional(in); err == nil {
		t.Error("didn't catch if without else")
	}

	in = "a if b else c"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "b ? a : c" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}

	in = "(1 - x / y) if z > 1 else 0"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "z > 1 ? (1 - x / y) : 0" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}

	in = "1 - a / b if c > 1 else 0"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "c > 1 ? 1 - a / b : 0" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}

	in = "1 - ( (a) if c else d )"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "1 - ( c ?  (a) : d )" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}

	// from SPR metrics -- TMA_....DRAM_Bound(%)
	in = "100 * ( min( ( ( ( a / ( b ) ) - ( min( ( ( ( ( 1 - ( ( ( 19 * ( c * ( 1 + ( d / e ) ) ) + 10 * ( ( f * ( 1 + ( d / e ) ) ) + ( g * ( 1 + ( d / e ) ) ) + ( h * ( 1 + ( d / e ) ) ) ) ) / ( ( 19 * ( c * ( 1 + ( d / e ) ) ) + 10 * ( ( f * ( 1 + ( d / e ) ) ) + ( g * ( 1 + ( d / e ) ) ) + ( h * ( 1 + ( d / e ) ) ) ) ) + ( 25 * ( ( i * ( 1 + ( d / e ) ) ) ) + 33 * ( ( j * ( 1 + ( d / e ) ) ) ) ) ) ) ) ) * ( a / ( b ) ) ) if ( ( 1000000 ) * ( j + i ) > e ) else 0 ) ) , ( 1 ) ) ) ) ) , ( 1 ) ) )"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "100 * ( min( ( ( ( a / ( b ) ) - ( min( ( ( ( ( 1000000 ) * ( j + i ) > e ) ?  ( ( 1 - ( ( ( 19 * ( c * ( 1 + ( d / e ) ) ) + 10 * ( ( f * ( 1 + ( d / e ) ) ) + ( g * ( 1 + ( d / e ) ) ) + ( h * ( 1 + ( d / e ) ) ) ) ) / ( ( 19 * ( c * ( 1 + ( d / e ) ) ) + 10 * ( ( f * ( 1 + ( d / e ) ) ) + ( g * ( 1 + ( d / e ) ) ) + ( h * ( 1 + ( d / e ) ) ) ) ) + ( 25 * ( ( i * ( 1 + ( d / e ) ) ) ) + 33 * ( ( j * ( 1 + ( d / e ) ) ) ) ) ) ) ) ) * ( a / ( b ) ) ) : 0 )  ) , ( 1 ) ) ) ) ) , ( 1 ) ) )" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}

	// from SPR metrics -- TMA_....Ports_Utilization(%)
	in = "100 * ( ( a + ( b / ( c ) ) * ( d - e ) + ( f + ( g / ( h + i + g + j ) ) * k ) ) / ( c ) if ( l < ( d - e ) ) else ( f + ( g / ( h + i + g + j ) ) * k ) / ( c ) )"
	if out, err = transformConditional(in); err != nil {
		t.Error(err)
	}
	if out != "100 * ( ( l < ( d - e ) ) ?  ( a + ( b / ( c ) ) * ( d - e ) + ( f + ( g / ( h + i + g + j ) ) * k ) ) / ( c ) : ( f + ( g / ( h + i + g + j ) ) * k ) / ( c ) )" {
		t.Errorf("improper transform: [%s] -> [%s]", in, out)
	}
}

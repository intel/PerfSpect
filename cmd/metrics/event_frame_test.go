package metrics

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import "testing"

func TestExtractInterval(t *testing.T) {
	tests := []struct {
		name string
		line []byte
		want float64
	}{
		{
			name: "ValidJSON",
			line: []byte(`{"interval" : 5.005073756, "cpu": "0"}`),
			want: 5.005073756,
		},
		{
			name: "ValidJSONWithSpaces",
			line: []byte(`{ "interval" : 42.12345 }`),
			want: 42.12345,
		},
		{
			name: "MissingInterval",
			line: []byte(`{"cpu": "0"}`),
			want: -1,
		},
		{
			name: "EmptyLine",
			line: []byte(``),
			want: -1,
		},
		{
			name: "InvalidNumber",
			line: []byte(`{"interval" : not_a_number, "cpu": "0"}`),
			want: -1,
		},
		{
			name: "IntervalAtEnd",
			line: []byte(`{"interval" : 123.456}`),
			want: 123.456,
		},
		{
			name: "IntervalWithTrailingSpace",
			line: []byte(`{"interval" : 77.88 , "cpu": "0"}`),
			want: 77.88,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInterval(tt.line)
			if got != tt.want {
				t.Errorf("extractInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"reflect"
	"testing"
)

func TestGetFrequenciesFromMSR(t *testing.T) {
	tests := []struct {
		name      string
		msr       string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid MSR with multiple frequencies",
			msr:       "0x1A2B3C4D",
			want:      []int{0x4D, 0x3C, 0x2B, 0x1A},
			expectErr: false,
		},
		{
			name:      "Valid MSR with single frequency",
			msr:       "0x1A",
			want:      []int{0x1A},
			expectErr: false,
		},
		{
			name:      "Empty MSR string",
			msr:       "",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Invalid MSR string",
			msr:       "invalid_hex",
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getFrequenciesFromMSR(tt.msr)
			if (err != nil) != tt.expectErr {
				t.Errorf("getFrequenciesFromMSR() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getFrequenciesFromMSR() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetBucketSizesFromMSR(t *testing.T) {
	tests := []struct {
		name      string
		msr       string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid MSR with 8 bucket sizes",
			msr:       "0x0102030405060708",
			want:      []int{8, 7, 6, 5, 4, 3, 2, 1},
			expectErr: false,
		},
		{
			name:      "Valid MSR with reversed order",
			msr:       "0x0807060504030201",
			want:      []int{1, 2, 3, 4, 5, 6, 7, 8},
			expectErr: false,
		},
		{
			name:      "Invalid MSR string",
			msr:       "invalid_hex",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "MSR with less than 8 bucket sizes",
			msr:       "0x01020304",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "MSR with more than 8 bucket sizes",
			msr:       "0x010203040506070809",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Empty MSR string",
			msr:       "",
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getBucketSizesFromMSR(tt.msr)
			if (err != nil) != tt.expectErr {
				t.Errorf("getBucketSizesFromMSR() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getBucketSizesFromMSR() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestExpandTurboFrequencies(t *testing.T) {
	tests := []struct {
		name      string
		buckets   [][]string
		isa       string
		want      []string
		expectErr bool
	}{
		{
			name: "Valid input with single bucket",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.5", "3.5"},
			expectErr: false,
		},
		{
			name: "Valid input with multiple buckets",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-2", "3.5", "3.2"},
				{"3-4", "3.6", "3.3"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.6", "3.6"},
			expectErr: false,
		},
		{
			name: "ISA column not found",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "3.5", "3.2"},
			},
			isa:       "AVX512",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Empty buckets",
			buckets: [][]string{
				{},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Invalid bucket range",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Empty frequency value",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{"1-4", "", "3.2"},
			},
			isa:       "SSE",
			want:      nil,
			expectErr: true,
		},
		{
			name: "Whitespace in bucket range",
			buckets: [][]string{
				{"Cores", "SSE", "AVX2"},
				{" 1-4 ", "3.5", "3.2"},
			},
			isa:       "SSE",
			want:      []string{"3.5", "3.5", "3.5", "3.5"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTurboFrequencies(tt.buckets, tt.isa)
			if (err != nil) != tt.expectErr {
				t.Errorf("expandTurboFrequencies() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandTurboFrequencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

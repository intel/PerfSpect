// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package telemetry

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestParsePDUOutletList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []int
		wantErr bool
	}{
		{name: "single outlet", value: "1", want: []int{1}},
		{name: "comma separated", value: "1,2,5", want: []int{1, 2, 5}},
		{name: "range", value: "1-8", want: []int{1, 2, 3, 4, 5, 6, 7, 8}},
		{name: "mixed", value: "1,4-6,9", want: []int{1, 4, 5, 6, 9}},
		{name: "single outlet range", value: "3-3", want: []int{3}},
		{name: "unsorted input is sorted", value: "9,2,5", want: []int{2, 5, 9}},
		{name: "duplicates removed", value: "1,1,2", want: []int{1, 2}},
		{name: "overlapping ranges deduplicated", value: "1-4,3-6", want: []int{1, 2, 3, 4, 5, 6}},
		{name: "surrounding whitespace tolerated", value: " 1 , 4 - 6 ", want: []int{1, 4, 5, 6}},
		{name: "max outlet accepted", value: "256", want: []int{256}},
		{name: "empty", value: "", wantErr: true},
		{name: "empty entry", value: "1,,2", wantErr: true},
		{name: "trailing comma", value: "1,2,", wantErr: true},
		{name: "non numeric", value: "a", wantErr: true},
		{name: "non numeric in range", value: "1-a", wantErr: true},
		{name: "descending range", value: "6-4", wantErr: true},
		{name: "too many range bounds", value: "1-2-3", wantErr: true},
		{name: "zero is not a valid outlet", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "above maximum", value: "257", wantErr: true},
		{name: "range above maximum", value: "1-99999", wantErr: true},
		// a mistyped bound must be rejected without first expanding the range, which
		// would otherwise allocate one entry per outlet in it
		{name: "absurd range bound", value: "1-2000000000", wantErr: true},
		{name: "command substitution rejected", value: "$(whoami)", wantErr: true},
		{name: "shell metacharacters rejected", value: "1;id", wantErr: true},
		{name: "path traversal rejected", value: "1/../outletgroup/1", wantErr: true},
		{name: "float rejected", value: "1.5", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePDUOutletList(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePDUOutletList(%q) = %v, want error", tt.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePDUOutletList(%q) returned unexpected error: %v", tt.value, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("parsePDUOutletList(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestPDUOutletPath(t *testing.T) {
	// outlet numbers are 1-based as labeled on the PDU, the model index is 0-based
	tests := []struct {
		outlet int
		want   string
	}{
		{outlet: 1, want: "outlet/0"},
		{outlet: 2, want: "outlet/1"},
		{outlet: 24, want: "outlet/23"},
	}
	for _, tt := range tests {
		if got := pduOutletPath(tt.outlet); got != tt.want {
			t.Errorf("pduOutletPath(%d) = %q, want %q", tt.outlet, got, tt.want)
		}
	}
}

func TestValidatePDUHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "ip address", host: "10.0.0.1"},
		{name: "hostname", host: "pdu-lab-01.example.com"},
		{name: "ip with port", host: "10.0.0.1:8443"},
		{name: "http prefix rejected", host: "http://10.0.0.1", wantErr: true},
		{name: "https prefix rejected", host: "https://10.0.0.1", wantErr: true},
		{name: "trailing path rejected", host: "10.0.0.1/model", wantErr: true},
		{name: "command substitution rejected", host: "$(whoami)", wantErr: true},
		{name: "backtick rejected", host: "10.0.0.1`id`", wantErr: true},
		{name: "semicolon rejected", host: "10.0.0.1;id", wantErr: true},
		{name: "whitespace rejected", host: "10.0.0.1 extra", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePDUHost(tt.host)
			if tt.wantErr && err == nil {
				t.Errorf("validatePDUHost(%q) = nil, want error", tt.host)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validatePDUHost(%q) returned unexpected error: %v", tt.host, err)
			}
		})
	}
}

func TestPDUConfigFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		wantEnabled   bool
		wantErr       bool
		wantReadPaths []string
	}{
		{
			name:        "not requested",
			env:         map[string]string{},
			wantEnabled: false,
		},
		{
			name: "outlet group only, the pre-existing configuration",
			env: map[string]string{
				pduHostEnvVar:     "10.0.0.1",
				pduUserEnvVar:     "admin",
				pduPasswordEnvVar: "secret",
				pduOutletEnvVar:   "3",
			},
			wantEnabled:   true,
			wantReadPaths: []string{"outletgroup/3"},
		},
		{
			name: "outlet list only",
			env: map[string]string{
				pduHostEnvVar:       "10.0.0.1",
				pduUserEnvVar:       "admin",
				pduPasswordEnvVar:   "secret",
				pduOutletListEnvVar: "1,2,5",
			},
			wantEnabled:   true,
			wantReadPaths: []string{"outlet/0", "outlet/1", "outlet/4"},
		},
		{
			name: "outlet list range",
			env: map[string]string{
				pduHostEnvVar:       "10.0.0.1",
				pduUserEnvVar:       "admin",
				pduPasswordEnvVar:   "secret",
				pduOutletListEnvVar: "1-3",
			},
			wantEnabled:   true,
			wantReadPaths: []string{"outlet/0", "outlet/1", "outlet/2"},
		},
		{
			name: "outlet list wins when both are set",
			env: map[string]string{
				pduHostEnvVar:       "10.0.0.1",
				pduUserEnvVar:       "admin",
				pduPasswordEnvVar:   "secret",
				pduOutletEnvVar:     "3",
				pduOutletListEnvVar: "1,2",
			},
			wantEnabled:   true,
			wantReadPaths: []string{"outlet/0", "outlet/1"},
		},
		{
			name: "missing host is not enabled",
			env: map[string]string{
				pduUserEnvVar:     "admin",
				pduPasswordEnvVar: "secret",
				pduOutletEnvVar:   "3",
			},
			wantEnabled: false,
		},
		{
			name: "missing password is not enabled",
			env: map[string]string{
				pduHostEnvVar:   "10.0.0.1",
				pduUserEnvVar:   "admin",
				pduOutletEnvVar: "3",
			},
			wantEnabled: false,
		},
		{
			name: "missing both outlet forms is not enabled",
			env: map[string]string{
				pduHostEnvVar:     "10.0.0.1",
				pduUserEnvVar:     "admin",
				pduPasswordEnvVar: "secret",
			},
			wantEnabled: false,
		},
		{
			name: "invalid host is an error",
			env: map[string]string{
				pduHostEnvVar:     "https://10.0.0.1",
				pduUserEnvVar:     "admin",
				pduPasswordEnvVar: "secret",
				pduOutletEnvVar:   "3",
			},
			wantErr: true,
		},
		{
			name: "invalid outlet group is an error",
			env: map[string]string{
				pduHostEnvVar:     "10.0.0.1",
				pduUserEnvVar:     "admin",
				pduPasswordEnvVar: "secret",
				pduOutletEnvVar:   "3;id",
			},
			wantErr: true,
		},
		{
			name: "invalid outlet list is an error",
			env: map[string]string{
				pduHostEnvVar:       "10.0.0.1",
				pduUserEnvVar:       "admin",
				pduPasswordEnvVar:   "secret",
				pduOutletListEnvVar: "1,bogus",
			},
			wantErr: true,
		},
		{
			name: "invalid outlet list is an error even when a valid group is set",
			env: map[string]string{
				pduHostEnvVar:       "10.0.0.1",
				pduUserEnvVar:       "admin",
				pduPasswordEnvVar:   "secret",
				pduOutletEnvVar:     "3",
				pduOutletListEnvVar: "bogus",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, envVar := range []string{pduHostEnvVar, pduUserEnvVar, pduPasswordEnvVar, pduOutletEnvVar, pduOutletListEnvVar} {
				t.Setenv(envVar, tt.env[envVar])
			}
			config, err := pduConfigFromEnv()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("pduConfigFromEnv() = %+v, want error", config)
				}
				if config.enabled() {
					t.Errorf("pduConfigFromEnv() returned an enabled config alongside an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("pduConfigFromEnv() returned unexpected error: %v", err)
			}
			if config.enabled() != tt.wantEnabled {
				t.Fatalf("pduConfigFromEnv().enabled() = %v, want %v", config.enabled(), tt.wantEnabled)
			}
			if !tt.wantEnabled {
				return
			}
			if !slices.Equal(config.readPaths, tt.wantReadPaths) {
				t.Errorf("pduConfigFromEnv().readPaths = %v, want %v", config.readPaths, tt.wantReadPaths)
			}
			if config.host != tt.env[pduHostEnvVar] {
				t.Errorf("pduConfigFromEnv().host = %q, want %q", config.host, tt.env[pduHostEnvVar])
			}
		})
	}
}

func TestPDUConfigBulkRequest(t *testing.T) {
	config := pduConfig{readPaths: []string{"outlet/0", "outlet/4"}}
	body, err := config.bulkRequest()
	if err != nil {
		t.Fatalf("bulkRequest() returned unexpected error: %v", err)
	}
	// the body must be valid JSON, since the PDU rejects anything else
	var decoded struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Requests []struct {
				RID  string `json:"rid"`
				JSON struct {
					Method string `json:"method"`
				} `json:"json"`
			} `json:"requests"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("bulkRequest() produced invalid JSON %q: %v", body, err)
	}
	if decoded.Method != "performBulk" {
		t.Errorf("bulkRequest() method = %q, want performBulk", decoded.Method)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("bulkRequest() jsonrpc = %q, want 2.0", decoded.JSONRPC)
	}
	if len(decoded.Params.Requests) != len(config.readPaths) {
		t.Fatalf("bulkRequest() produced %d sub-requests, want %d", len(decoded.Params.Requests), len(config.readPaths))
	}
	wantRIDs := []string{"/model/outlet/0/activePower", "/model/outlet/4/activePower"}
	for i, request := range decoded.Params.Requests {
		if request.RID != wantRIDs[i] {
			t.Errorf("sub-request %d rid = %q, want %q", i, request.RID, wantRIDs[i])
		}
		if request.JSON.Method != "getReading" {
			t.Errorf("sub-request %d method = %q, want getReading", i, request.JSON.Method)
		}
	}
}

// TestPDUBulkRequestIsShellSafe guards the script parameter that is interpolated into a
// single-quoted bash string on the target.
func TestPDUBulkRequestIsShellSafe(t *testing.T) {
	var readPaths []string
	for outlet := 1; outlet <= maxPDUOutletNumber; outlet++ {
		readPaths = append(readPaths, pduOutletPath(outlet))
	}
	body, err := pduConfig{readPaths: readPaths}.bulkRequest()
	if err != nil {
		t.Fatalf("bulkRequest() returned unexpected error: %v", err)
	}
	for _, unsafe := range []string{"'", "\\", "\n"} {
		if strings.Contains(body, unsafe) {
			t.Errorf("bulk request contains unsafe substring %q", unsafe)
		}
	}
}

// TestPDUReadPathsAreShellSafe guards the resource paths that are embedded in the
// request body sent to the PDU.
func TestPDUReadPathsAreShellSafe(t *testing.T) {
	t.Setenv(pduHostEnvVar, "10.0.0.1")
	t.Setenv(pduUserEnvVar, "admin")
	t.Setenv(pduPasswordEnvVar, "secret")
	t.Setenv(pduOutletEnvVar, "")
	t.Setenv(pduOutletListEnvVar, "1-24")
	config, err := pduConfigFromEnv()
	if err != nil {
		t.Fatalf("pduConfigFromEnv() returned unexpected error: %v", err)
	}
	joined := strings.Join(config.readPaths, " ")
	for _, unsafe := range []string{"$", "`", ";", "&", "|", "<", ">", "(", ")", "\"", "'", "\\", "\n", "\t", ".."} {
		if strings.Contains(joined, unsafe) {
			t.Errorf("read paths %q contain unsafe substring %q", joined, unsafe)
		}
	}
}

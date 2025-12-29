// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"reflect"
	"testing"
)

func TestGetSectionsFromOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name: "Valid sections with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2
########## Section C ##########
Content C1`,
			want: map[string]string{
				"Section A": "Content A1\nContent A2\n",
				"Section B": "Content B1\nContent B2\n",
				"Section C": "Content C1\n",
			},
		},
		{
			name: "Valid sections with empty content",
			output: `########## Section A ##########
########## Section B ##########
########## Section C ##########`,
			want: map[string]string{
				"Section A": "",
				"Section B": "",
				"Section C": "",
			},
		},
		{
			name:   "No sections",
			output: "No section headers here",
			want:   map[string]string{},
		},
		{
			name:   "Empty output",
			output: ``,
			want:   map[string]string{},
		},
		{
			name:   "Empty lines in output",
			output: "\n\n\n",
			want:   map[string]string{},
		},
		{
			name: "Section with trailing newlines",
			output: `########## Section A ##########

Content A1

########## Section B ##########
Content B1`,
			want: map[string]string{
				"Section A": "\nContent A1\n\n",
				"Section B": "Content B1\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSectionsFromOutput(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSectionsFromOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSectionValueFromOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		sectionName string
		want        string
	}{
		{
			name: "Section A exists with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2`,
			sectionName: "Section A",
			want:        "Content A1\nContent A2\n",
		},
		{
			name: "Section B exists with content",
			output: `########## Section A ##########
Content A1
Content A2
########## Section B ##########
Content B1
Content B2`,
			sectionName: "Section B",
			want:        "Content B1\nContent B2\n",
		},
		{
			name: "Section exists with no content",
			output: `########## Section A ##########
########## Section B ##########
Content B1`,
			sectionName: "Section A",
			want:        "",
		},
		{
			name: "Section does not exist",
			output: `########## Section A ##########
Content A1
########## Section B ##########
Content B1`,
			sectionName: "Section C",
			want:        "",
		},
		{
			name:        "Empty output",
			output:      "",
			sectionName: "Section A",
			want:        "",
		},
		{
			name: "Section with trailing newlines",
			output: `########## Section A ##########

Content A1

########## Section B ##########
Content B1`,
			sectionName: "Section A",
			want:        "\nContent A1\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SectionValueFromOutput(tt.output, tt.sectionName)
			if got != tt.want {
				t.Errorf("sectionValueFromOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

package report

import (
	"reflect"
	"testing"
)

func TestConvertHexStringToDecimals(t *testing.T) {
	tests := []struct {
		name      string
		hexStr    string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid hex string with 16 characters",
			hexStr:    "1212121212121212",
			want:      []int{18, 18, 18, 18, 18, 18, 18, 18},
			expectErr: false,
		},
		{
			name:      "Valid hex string with 0x prefix",
			hexStr:    "0x1212121212121212",
			want:      []int{18, 18, 18, 18, 18, 18, 18, 18},
			expectErr: false,
		},
		{
			name:      "Hex string shorter than 16 characters",
			hexStr:    "12121212",
			want:      []int{18, 18, 18, 18, 0, 0, 0, 0},
			expectErr: false,
		},
		{
			name:      "Hex string longer than 16 characters",
			hexStr:    "121212121212121212",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Invalid hex string with non-hex characters",
			hexStr:    "1212G212",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Empty hex string",
			hexStr:    "",
			want:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertHexStringToDecimals(tt.hexStr)
			if (err != nil) != tt.expectErr {
				t.Errorf("convertHexStringToDecimals() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertHexStringToDecimals() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestExpandCPUList(t *testing.T) {
	tests := []struct {
		name      string
		cpuList   string
		want      []int
		expectErr bool
	}{
		{
			name:      "Valid single CPU",
			cpuList:   "3",
			want:      []int{3},
			expectErr: false,
		},
		{
			name:      "Valid range of CPUs",
			cpuList:   "1-3",
			want:      []int{1, 2, 3},
			expectErr: false,
		},
		{
			name:      "Valid mixed single and range",
			cpuList:   "1,3-5,8",
			want:      []int{1, 3, 4, 5, 8},
			expectErr: false,
		},
		{
			name:      "Empty CPU list",
			cpuList:   "",
			want:      []int{},
			expectErr: false,
		},
		{
			name:      "Invalid CPU range",
			cpuList:   "1-",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Invalid CPU number",
			cpuList:   "1,a",
			want:      nil,
			expectErr: true,
		},
		{
			name:      "Overlapping ranges",
			cpuList:   "1-3,2-4",
			want:      []int{1, 2, 3, 2, 3, 4},
			expectErr: false,
		},
		{
			name:      "Whitespace in input",
			cpuList:   " 1 , 3-5 ",
			want:      []int{1, 3, 4, 5},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandCPUList(tt.cpuList)
			if (err != nil) != tt.expectErr {
				t.Errorf("expandCPUList() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandCPUList() = %v, want %v", got, tt.want)
			}
		})
	}
}

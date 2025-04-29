package target

import (
	"os"
	"strings"
	"testing"
)

type MockLocalTarget struct {
	LocalTarget
}

func TestGetUserPath(t *testing.T) {
	// Backup and defer restore of original PATH environment variable
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	tests := []struct {
		name          string
		envPath       string
		expectedPaths []string
	}{
		{
			name:          "Valid paths in PATH",
			envPath:       "/usr/bin:/bin:/usr/local/bin",
			expectedPaths: []string{"/usr/bin", "/bin", "/usr/local/bin"},
		},
		{
			name:          "Invalid paths in PATH",
			envPath:       "/invalid/path:/another/invalid:/usr/bin",
			expectedPaths: []string{"/usr/bin"},
		},
		{
			name:          "Empty PATH",
			envPath:       "",
			expectedPaths: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the PATH environment variable for the test
			os.Setenv("PATH", tt.envPath)

			// Create a mock LocalTarget
			mockTarget := &MockLocalTarget{}

			// Call GetUserPath
			result, err := mockTarget.GetUserPath()
			if err != nil {
				t.Fatalf("GetUserPath returned an error: %v", err)
			}

			// Split the result into paths
			resultPaths := strings.Split(result, ":") // returns a slice containing a single empty string, if result is empty
			if len(resultPaths) == 1 && resultPaths[0] == "" {
				resultPaths = []string{}
			}

			// Compare the result with the expected paths
			if len(resultPaths) != len(tt.expectedPaths) {
				t.Errorf("Expected %d paths, got %d", len(tt.expectedPaths), len(resultPaths))
			}

			for _, expectedPath := range tt.expectedPaths {
				found := false
				for _, resultPath := range resultPaths {
					if resultPath == expectedPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected path %s not found in result", expectedPath)
				}
			}
		})
	}
}

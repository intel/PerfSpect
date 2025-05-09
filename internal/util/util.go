/*
Package util includes utility/helper functions that may be useful to other modules.
*/
package util

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ExpandUser expands '~' to user's home directory, if found, otherwise returns original path
func ExpandUser(path string) string {
	usr, _ := user.Current()
	if path == "~" {
		return usr.HomeDir
	} else if strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
		return filepath.Join(usr.HomeDir, path[2:])
	} else {
		return path
	}
}

// AbsPath returns absolute path after expanding '~' to user's home dir
// Useful when application is started by a process that isn't a shell, e.g. PKB
// Use everywhere in place of filepath.Abs()
func AbsPath(path string) (string, error) {
	return filepath.Abs(ExpandUser(path))
}

// FileExists checks if a file exists at the given path.
// It returns a boolean indicating whether the file exists, and an error if the
// path refers to a non-regular file, e.g., a directory.
func FileExists(path string) (exists bool, err error) {
	var fileInfo fs.FileInfo
	fileInfo, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
			err = nil
			return
		}
		return
	}
	if !fileInfo.Mode().IsRegular() {
		err = fmt.Errorf("%s not a file", path)
		return
	}
	exists = true
	return
}

// DirectoryExists checks if the specified directory exists.
// It returns a boolean indicating whether the directory exists and an error if the
// path refers to anything other than a directory, e.g., a regular file.
func DirectoryExists(path string) (exists bool, err error) {
	var fileInfo fs.FileInfo
	fileInfo, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
			err = nil
			return
		}
		return
	}
	if !fileInfo.Mode().IsDir() {
		err = fmt.Errorf("%s not a directory", path)
		return
	}
	exists = true
	return
}

// IsValidDirectoryName checks if the provided string is a valid directory name.
// A valid directory name can contain alphanumeric characters, dots (.), underscores (_),
// forward slashes (/), and hyphens (-). It must match the regular expression `^[a-zA-Z0-9._/-]+$`.
//
// Parameters:
//   - name: The directory name to validate.
//
// Returns:
//   - true if the directory name is valid, false otherwise.
func IsValidDirectoryName(name string) bool {
	// Regular expression to match valid directory names
	re := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	return re.MatchString(name)
}

// CopyDirectory copies the contents of a directory from the source path to the destination path.
// It recursively copies all subdirectories and files within the directory.
// The function returns an error if any error occurs during the copying process.
func CopyDirectory(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if fileInfo.Mode().IsDir() {
			// Create the subdirectory in the destination directory
			if err := CreateDirectoryIfNotExists(destPath, 0755); err != nil {
				return err
			}
			// Recursively copy the contents of the subdirectory
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		} else if fileInfo.Mode().IsRegular() {
			// Copy the file to the destination directory
			if err := CopyFile(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a file from the source path to the destination path.
// If the destination path is a directory, the file will be copied with the same name to that directory.
// The file permissions of the source file will be preserved in the destination file.
func CopyFile(srcFile, dstFile string) error {
	// Open the source file
	srcFileStat, err := os.Stat(srcFile)
	if err != nil {
		return err
	}
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()
	// Create the destination file
	dstFileStat, err := os.Stat(dstFile)
	if err == nil && dstFileStat.IsDir() {
		dstFile = filepath.Join(dstFile, filepath.Base(srcFile))
	}
	dest, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	// Copy the contents of the source file to the destination file
	_, err = io.Copy(dest, src)
	dest.Close()
	if err != nil {
		return err
	}
	// Preserve the file permissions of the source file in the destination file
	err = os.Chmod(dstFile, srcFileStat.Mode())
	return err
}

// FileOrDirectoryExists checks if a file or directory exists at the given file path.
// It returns true if the file or directory exists, and false otherwise.
func FileOrDirectoryExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// CreateDirectoryIfNotExists creates a directory at the specified path if it does not already exist.
// If the directory already exists, it does nothing and returns nil.
// If there is an error while creating the directory, it returns an error with a descriptive message.
func CreateDirectoryIfNotExists(dir string, perm os.FileMode) error {
	if FileOrDirectoryExists(dir) {
		return nil
	}
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}
	return nil
}

// GeoMean calculates the geomean of a slice of floats
func GeoMean(vals []float64) (val float64) {
	m := 0.0
	for i, x := range vals {
		lx := math.Log(x)
		m += (lx - m) / float64(i+1)
	}
	val = math.Exp(m)
	return
}

// ExtractResource extracts a resource from the given embed.FS and saves it to the specified temporary directory.
// It returns the path to the saved resource file and any error encountered during the process.
func ExtractResource(resources embed.FS, resourcePath string, tempDir string) (string, error) {
	var outPath string
	var resourceBytes []byte
	isDir := false
	resourceBytes, err := resources.ReadFile(resourcePath)
	if err != nil {
		if strings.Contains(err.Error(), "is a directory") {
			isDir = true
		} else {
			return "", err
		}
	}
	if isDir {
		dirEntries, err := resources.ReadDir(resourcePath)
		if err != nil {
			return "", err
		}
		resourceName := filepath.Base(resourcePath)
		outPath = filepath.Join(tempDir, resourceName)
		err = os.Mkdir(outPath, 0755)
		if err != nil {
			return "", err
		}
		for _, entry := range dirEntries {
			// Recursively extract resources from subdirectories
			_, err = ExtractResource(resources, filepath.Join(resourcePath, entry.Name()), outPath)
			if err != nil {
				return "", err
			}
		}
	} else {
		// write the resource to a file in the temp directory
		resourceName := filepath.Base(resourcePath)
		outPath = filepath.Join(tempDir, resourceName)
		var f *os.File
		f, err = os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0744)
		if err != nil {
			return "", err
		}
		defer f.Close()
		err = binary.Write(f, binary.LittleEndian, resourceBytes)
		if err != nil {
			return "", err
		}
	}
	return outPath, nil
}

// UniqueAppend appends an item to a slice if it is not already present
func UniqueAppend[T comparable](slice []T, item T) []T {
	if slices.Contains(slice, item) {
		return slice
	}
	return append(slice, item)
}

// CompareVersions compares two version strings
// version format: major.minor.patch<-alpha|beta|rc><.build>
// examples: 1.2.3, 1.2.3-alpha.4
// Returns
// -1 if v1 is less than v2
// 0 if v1 is equal to v2
// 1 if v1 is greater than v2
// An error if the version strings are not valid
func CompareVersions(v1, v2 string) (int, error) {
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)[-]?(alpha|beta|rc)?[\.]?(\d+)?`)
	v1Parts := re.FindStringSubmatch(v1)
	if v1Parts == nil {
		return 0, fmt.Errorf("error: unable to parse version string: %s", v1)
	}
	v2Parts := re.FindStringSubmatch(v2)
	if v2Parts == nil {
		return 0, fmt.Errorf("error: unable to parse version string: %s", v2)
	}
	// compare version parts
	for i := 1; i < 6; i++ {
		if i == 4 {
			v1Part := v1Parts[i]
			v2Part := v2Parts[i]
			// compare alpha, beta, rc
			if v1Part == "" && v2Part == "" {
				return 0, nil
			} else if v1Part == "" && v2Part != "" { // v2 is tagged with alpha, beta, rc
				return 1, nil
			} else if v1Part != "" && v2Part == "" { // v1 is tagged with alpha, beta, rc
				return -1, nil
			} else { // both v1 and v2 are tagged with alpha, beta, rc
				intVals := map[string]int{"alpha": 1, "beta": 2, "rc": 3}
				if intVals[v1Part] > intVals[v2Part] {
					return 1, nil
				} else if intVals[v1Part] < intVals[v2Part] {
					return -1, nil
				}
			}
			continue
		}
		v1Part, err := strconv.Atoi(v1Parts[i])
		if err != nil {
			return 0, err
		}
		v2Part, err := strconv.Atoi(v2Parts[i])
		if err != nil {
			return 0, err
		}
		if v1Part > v2Part {
			return 1, nil
		} else if v1Part < v2Part {
			return -1, nil
		}
	}
	// The version strings are equal
	return 0, nil
}

// ExtractTGZ extracts the contents of a tarball (.tar.gz) file to the specified destination directory.
// If stripComponent is true, the first directory in the tarball will be skipped.
func ExtractTGZ(tarballPath, destDir string, stripComponent bool) error {
	// Open the tarball
	tarball, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer tarball.Close()
	gzipReader, err := gzip.NewReader(tarball)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	// Create a new tar reader
	tarReader := tar.NewReader(gzipReader)

	targetIdx := 0
	firstDirectory := ""
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Check for invalid paths, there should never be a ".." in the path
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("tarball contains invalid path: %s", header.Name)
		}

		target := filepath.Join(destDir, header.Name)

		if stripComponent {
			// Skip the first directory in the tarball
			if targetIdx == 0 && header.Typeflag != tar.TypeDir {
				return fmt.Errorf("first entry in tarball is not a directory")
			}
			if targetIdx == 0 {
				firstDirectory = header.Name
				targetIdx++
				continue
			} else if targetIdx > 0 {
				// remove the first directory from the target path
				target = filepath.Join(destDir, strings.TrimPrefix(header.Name, firstDirectory))
			}
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tarReader); err != nil { // nosemgrep
				f.Close()
				return err
			}
			f.Close()
		}
		targetIdx++
	}
	return nil
}

// GetAppDir returns the directory of the executable
func GetAppDir() string {
	exePath, _ := os.Executable()
	return filepath.Dir(exePath)
}

// SignalChildren sends a signal to all children of this process
func SignalChildren(sig os.Signal) {
	// get list of child processes
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(os.Getpid()))
	out, err := cmd.Output()
	if err != nil {
		slog.Error("failed to get child processes", slog.String("error", err.Error()))
		return
	}
	// send signal to each child
	for pid := range strings.SplitSeq(string(out), "\n") {
		if pid == "" {
			continue
		}
		pidInt, err := strconv.Atoi(pid)
		if err != nil {
			slog.Error("failed to convert pid to int", slog.String("pid", pid), slog.String("error", err.Error()))
			continue
		}
		proc, err := os.FindProcess(pidInt)
		if err != nil {
			slog.Error("failed to find process", slog.Int("pid", pidInt), slog.String("error", err.Error()))
			continue
		}
		slog.Info("sending signal to child process", slog.Int("pid", pidInt), slog.String("signal", sig.String()))
		err = proc.Signal(sig)
		if err != nil {
			slog.Error("failed to send signal to process", slog.Int("pid", pidInt), slog.String("error", err.Error()))
		}
	}
}

// IsValidHex checks if a string is a valid hex string
// Valid hex strings are non-empty, optionally prefixed with "0x" or "0X",
// and contain only valid hex characters (0-9, a-f, A-F).
func IsValidHex(hexStr string) bool {
	// Check if the string starts with "0x" or "0X"
	if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
		hexStr = hexStr[2:]
	}
	// Check if the string can be parsed as a hex number
	_, err := strconv.ParseUint(hexStr, 16, 64)
	return err == nil
}

// HexToIntList converts hex string to a list of integers 16 bits (2 hex chars)
// at a time. The hex string can, optionally, be prefixed with "0x" or "0X".
// For example, "0x1234", "0X1234", and "1234" will be converted to [0x12, 0x34].
// If the hex string is not valid, an error is returned.
func HexToIntList(hexStr string) ([]int, error) {
	if !IsValidHex(hexStr) {
		return nil, fmt.Errorf("invalid hex string: %s", hexStr)
	}
	// Remove the "0x" or "0X" prefix if present
	if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
		hexStr = hexStr[2:]
	}
	// Pad the hex string with a leading zero if necessary
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	// Convert the hex string to a list of integers
	intList := make([]int, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		// Convert each pair of hex characters to an integer
		val, err := strconv.ParseInt(hexStr[i:i+2], 16, 16)
		if err != nil {
			return nil, fmt.Errorf("failed to convert hex to int: %s", err)
		}
		intList[i/2] = int(val)
	}
	return intList, nil
}

// IntRangeToIntList expands a string representing a range of integers into a slice of integers.
// The function returns a slice of integers representing the expanded range.
// For example, "1-3" will be expanded to [1, 2, 3]. And, "5" will be expanded to [5].
// If the input string is not in a valid format, it returns an error.
func IntRangeToIntList(input string) ([]int, error) {
	// check input format matches "start-end", or "start"
	re := regexp.MustCompile(`^(\d+)(?:-(\d+))?$`)
	matches := re.FindStringSubmatch(input)
	if len(matches) == 0 {
		err := fmt.Errorf("invalid input format: %s", input)
		return nil, err
	}
	start, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid start value: %s", matches[1])
	}
	// if end value is empty, return a slice with the start value
	if matches[2] == "" {
		return []int{start}, nil
	}
	// if end value is provided, parse it
	end, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid end value: %s", matches[2])
	}
	if start > end {
		return nil, fmt.Errorf("start value is greater than end value: %d > %d", start, end)
	}
	// create a slice of integers from start to end
	result := make([]int, end-start+1)
	for i := start; i <= end; i++ {
		result[i-start] = i
	}
	return result, nil
}

// SelectiveIntRangeToIntList expands a string representing a selective range of integers into a slice of integers.
// For example "1-3,7,9,11-13" will be expanded to [1, 2, 3, 7, 9, 11, 12, 13].
// An error is returned if the input string is not in a valid format.
func SelectiveIntRangeToIntList(input string) ([]int, error) {
	var result []int
	for r := range strings.SplitSeq(input, ",") {
		ints, err := IntRangeToIntList(r)
		if err != nil {
			return nil, err
		}
		result = append(result, ints...)
	}
	return result, nil
}

// IntSliceToStringSlice converts a slice of integers to a slice of strings.
func IntSliceToStringSlice(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}

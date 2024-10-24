/*
Package util includes utility/helper functions that may be useful to other modules.
*/
package util

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
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
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			// Recursively copy the contents of the subdirectory
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		} else if fileInfo.Mode().IsRegular() {
			// Copy the file to the destination directory
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// Copy copies a file from the source path to the destination path.
// If the destination path is a directory, the file will be copied with the same name to that directory.
// The file permissions of the source file will be preserved in the destination file.
func Copy(srcFile, dstFile string) error {
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

// Exists checks if a file or directory exists at the given file path.
// It returns true if the file or directory exists, and false otherwise.
func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// CreateIfNotExists creates a directory at the specified path if it does not already exist.
// If the directory already exists, it does nothing and returns nil.
// If there is an error while creating the directory, it returns an error with a descriptive message.
func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}
	return nil
}

// StringIndexInList returns the index of the given string in the given list of
// strings and error if not found
func StringIndexInList(s string, l []string) (idx int, err error) {
	var item string
	for idx, item = range l {
		if item == s {
			return
		}
	}
	err = fmt.Errorf("%s not found in %s", s, strings.Join(l, ", "))
	return
}

// StringInList confirms if string is in list of strings
func StringInList(s string, l []string) bool {
	for _, item := range l {
		if item == s {
			return true
		}
	}
	return false
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
func UniqueAppend(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
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
			if _, err := io.Copy(f, tarReader); err != nil {
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

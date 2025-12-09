package common

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path"
	"perfspect/internal/cpus"
	"perfspect/internal/progress"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"slices"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

// target flags
var (
	flagTargetHost    string
	flagTargetPort    string
	flagTargetUser    string
	flagTargetKeyFile string
	flagTargetsFile   string
)

// target flag names
const (
	flagTargetsFileName = "targets"
	flagTargetHostName  = "target"
	flagTargetPortName  = "port"
	flagTargetUserName  = "user"
	flagTargetKeyName   = "key"
)

var targetFlags = []Flag{
	{Name: flagTargetHostName, Help: "host name or IP address of remote target"},
	{Name: flagTargetPortName, Help: "port for SSH to remote target"},
	{Name: flagTargetUserName, Help: "user name for SSH to remote target"},
	{Name: flagTargetKeyName, Help: "private key file for SSH to remote target"},
	{Name: flagTargetsFileName, Help: "file with remote target(s) connection details. See targets.yaml for format."},
}

func AddTargetFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagTargetHost, flagTargetHostName, "", targetFlags[0].Help)
	cmd.Flags().StringVar(&flagTargetPort, flagTargetPortName, "", targetFlags[1].Help)
	cmd.Flags().StringVar(&flagTargetUser, flagTargetUserName, "", targetFlags[2].Help)
	cmd.Flags().StringVar(&flagTargetKeyFile, flagTargetKeyName, "", targetFlags[3].Help)
	cmd.Flags().StringVar(&flagTargetsFile, flagTargetsFileName, "", targetFlags[4].Help)

	cmd.MarkFlagsMutuallyExclusive(flagTargetHostName, flagTargetsFileName)
}

func GetTargetFlagGroup() FlagGroup {
	return FlagGroup{
		GroupName: "Remote Target Options",
		Flags:     targetFlags,
	}
}

func ValidateTargetFlags(cmd *cobra.Command) error {
	if flagTargetsFile != "" && flagTargetHost != "" {
		return fmt.Errorf("only one of --%s or --%s can be specified", flagTargetsFileName, flagTargetHostName)
	}
	if flagTargetsFile != "" && (flagTargetPort != "" || flagTargetUser != "" || flagTargetKeyFile != "") {
		return fmt.Errorf("if --%s is specified, --%s, --%s, and --%s must not be specified", flagTargetsFileName, flagTargetPortName, flagTargetUserName, flagTargetKeyName)
	}
	if (flagTargetPort != "" || flagTargetUser != "" || flagTargetKeyFile != "") && flagTargetHost == "" {
		return fmt.Errorf("if --%s, --%s, or --%s is specified, --%s must also be specified", flagTargetPortName, flagTargetUserName, flagTargetKeyName, flagTargetHostName)
	}
	// confirm that the targets file exists
	if flagTargetsFile != "" {
		if _, err := os.Stat(flagTargetsFile); os.IsNotExist(err) {
			return fmt.Errorf("targets file %s does not exist", flagTargetsFile)
		}
	}
	// confirm that port is a positive integer
	if flagTargetPort != "" {
		var port int
		var err error
		if port, err = strconv.Atoi(flagTargetPort); err != nil || port <= 0 {
			return fmt.Errorf("port %s is not a positive integer", flagTargetPort)
		}
	}
	// confirm that the key file exists
	if flagTargetKeyFile != "" {
		if _, err := os.Stat(flagTargetKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file %s does not exist", flagTargetKeyFile)
		}
	}
	// confirm that user is a valid user name
	if flagTargetUser != "" {
		userNameRegex := `^[a-z_][a-z0-9_.-]{0,63}$`
		re := regexp.MustCompile(userNameRegex)
		if !re.MatchString(flagTargetUser) {
			return fmt.Errorf("user name %s does not match the user name regex '%s'", flagTargetUser, userNameRegex)
		}
	}
	// confirm that host is a valid host name or IP address
	if flagTargetHost != "" {
		hostNameRegex := `^([a-zA-Z0-9.-]+)$`
		re := regexp.MustCompile(hostNameRegex)
		if !re.MatchString(flagTargetHost) {
			return fmt.Errorf("host name %s does not match the host name regex '%s'", flagTargetHost, hostNameRegex)
		}
	}
	return nil
}

// GetTargets retrieves the list of targets based on the provided command and parameters. It creates
// a temporary directory for each target and returns a slice of target.Target objects.
func GetTargets(cmd *cobra.Command, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (targets []target.Target, targetErrs []error, err error) {
	tempDirFlag := cmd.Parent().PersistentFlags().Lookup("tempdir")
	if tempDirFlag == nil {
		// try grand-parent command (in case this is a subcommand)
		grandParent := cmd.Parent().Parent()
		if grandParent != nil {
			tempDirFlag = grandParent.PersistentFlags().Lookup("tempdir")
		}
	}
	if tempDirFlag == nil {
		err = fmt.Errorf("failed to find 'tempdir' persistent flag")
		slog.Error(err.Error())
		return
	}
	targetTempDirRoot := tempDirFlag.Value.String()
	flagTargetsFile, _ := cmd.Flags().GetString(flagTargetsFileName)
	if flagTargetsFile != "" {
		targets, targetErrs, err = getTargetsFromFile(flagTargetsFile, localTempDir)
	} else {
		myTarget, targetErr, functionErr := getSingleTarget(cmd, needsElevatedPrivileges, failIfCantElevate, localTempDir)
		targets = []target.Target{myTarget}
		targetErrs = []error{targetErr}
		err = functionErr
	}
	if err != nil {
		slog.Error("failed to get targets", slog.String("error", err.Error()))
		return
	}
	// create a temp directory on each target
	for targetIdx, myTarget := range targets {
		// if we already have an error for this target, skip it
		if targetErrs[targetIdx] != nil {
			continue
		}
		_, err := myTarget.CreateTempDirectory(targetTempDirRoot)
		if err != nil {
			targetErrs[targetIdx] = fmt.Errorf("failed to create temp directory on target: %v", err)
			slog.Error(targetErrs[targetIdx].Error(), slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			continue
		}
		// confirm that the temp directory was created on a file system that was not mounted with noexec
		noExec, err := isDirNoExec(myTarget, myTarget.GetTempDirectory())
		if err != nil {
			// log the error but don't reject the target just in case our check is wrong
			slog.Warn("failed to check if temp directory is mounted on 'noexec' file system", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
		} else if noExec {
			targetErrs[targetIdx] = fmt.Errorf("target's temp directory must not be on a file system mounted with the 'noexec' option, override the default with --tempdir")
			slog.Error(targetErrs[targetIdx].Error(), slog.String("target", myTarget.GetName()))
		}
	}
	return
}

// getSingleTarget returns a target.Target object representing the target host and associated details.
// The function takes the following parameters:
// - cmd: A pointer to the cobra.Command object representing the command.
// - needsElevatedPriviliges: A boolean indicating whether elevated privileges are required.
// - failIfCantElevate: A boolean indicating whether to fail if elevated privileges can't be obtained.
// - localTempDir: A string representing the local temporary directory.
// The function returns the following values:
// - myTarget: A target.Target object representing the target host and associated details.
// - targetError: An error indicating a problem with the target host connection.
// - err: An error object indicating any error that occurred during the function execution.
func getSingleTarget(cmd *cobra.Command, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (target.Target, error, error) {
	targetHost, _ := cmd.Flags().GetString(flagTargetHostName)
	targetPort, _ := cmd.Flags().GetString(flagTargetPortName)
	targetUser, _ := cmd.Flags().GetString(flagTargetUserName)
	targetKey, _ := cmd.Flags().GetString(flagTargetKeyName)
	if targetHost != "" {
		return getRemoteTarget(targetHost, targetPort, targetUser, targetKey, needsElevatedPrivileges, failIfCantElevate, localTempDir)
	} else {
		return getLocalTarget(needsElevatedPrivileges, failIfCantElevate, localTempDir)
	}
}

// getLocalTarget creates a new local target object.
func getLocalTarget(needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (target.Target, error, error) {
	myTarget := target.NewLocalTarget()
	// create a sub-directory for the target in the localTempDir
	localTargetDir := path.Join(localTempDir, myTarget.GetName())
	err := os.MkdirAll(localTargetDir, 0700)
	if err != nil {
		return myTarget, nil, err
	}
	if needsElevatedPrivileges && !myTarget.CanElevatePrivileges() {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			slog.Warn("can not prompt for sudo password because STDIN isn't coming from a terminal")
			if failIfCantElevate {
				err := fmt.Errorf("failed to elevate privileges on local target")
				return myTarget, err, nil
			} else {
				slog.Warn("continuing without elevated privileges")
			}
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: some operations cannot be performed without elevated privileges.\n")
			currentUser, err := user.Current()
			if err != nil {
				return myTarget, nil, err
			}
			fmt.Fprintf(os.Stderr, "For complete functionality, please provide your password at the prompt.\n")
			slog.Info("prompting for sudo password")
			prompt := fmt.Sprintf("[sudo] password for %s", currentUser.Username)
			var sudoPwd string
			sudoPwd, err = getPassword(prompt)
			if err != nil {
				return myTarget, nil, err
			}
			myTarget.SetSudo(sudoPwd)
			if !myTarget.CanElevatePrivileges() {
				if failIfCantElevate {
					err := fmt.Errorf("failed to elevate privileges on local target")
					return myTarget, nil, err
				} else {
					slog.Warn("failed to elevate privileges on local target, continuing without elevated privileges")
					fmt.Fprintf(os.Stderr, "WARNING: Not able to establish elevated privileges with provided password.\n")
					fmt.Fprintf(os.Stderr, "Continuing with regular user privileges. Some operations will not be performed.\n")
				}
			}
		}
	}
	return myTarget, nil, nil
}

// getRemoteTarget creates a new remote target object based on the provided parameters.
func getRemoteTarget(targetHost string, targetPort string, targetUser string, targetKey string, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (target.Target, error, error) {
	// if targetPort is empty, default to 22
	if targetPort == "" {
		targetPort = "22"
	}
	slog.Debug("Creating remote target", slog.String("targetHost", targetHost), slog.String("targetPort", targetPort), slog.String("targetUser", targetUser))
	myTarget := target.NewRemoteTarget(targetHost, targetHost, targetPort, targetUser, targetKey)
	// create a sub-directory for the target in the localTempDir
	localTargetDir := path.Join(localTempDir, myTarget.GetName())
	err := os.MkdirAll(localTargetDir, 0700)
	if err != nil {
		return myTarget, nil, err
	}
	if !myTarget.CanConnect() {
		if targetKey == "" && targetUser != "" {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				err := fmt.Errorf("can not prompt for SSH password because STDIN isn't coming from a terminal")
				slog.Error(err.Error())
				return myTarget, nil, err
			} else {
				slog.Info("Prompting for SSH password.", slog.String("targetHost", targetHost), slog.String("targetPort", targetPort), slog.String("targetUser", targetUser))
				sshPwd, err := getPassword(fmt.Sprintf("%s@%s's password", targetUser, targetHost))
				if err != nil {
					return myTarget, nil, err
				}
				var hostArchitecture string
				hostArchitecture, err = getHostArchitecture()
				if err != nil {
					return myTarget, nil, err
				}
				sshPassPath, err := util.ExtractResource(script.Resources, path.Join("resources", hostArchitecture, "sshpass"), localTargetDir)
				if err != nil {
					return myTarget, nil, err
				}
				myTarget.SetSshPassPath(sshPassPath)
				myTarget.SetSshPass(sshPwd)
				// if still can't connect, return target error
				if !myTarget.CanConnect() {
					err = fmt.Errorf("failed to connect to target host (%s)", myTarget.GetName())
					return myTarget, err, nil
				}
			}
		} else {
			err := fmt.Errorf("failed to connect to target host (%s)", myTarget.GetName())
			return myTarget, nil, err
		}
	}
	if needsElevatedPrivileges && !myTarget.CanElevatePrivileges() {
		if failIfCantElevate {
			err := fmt.Errorf("failed to elevate privileges on remote target")
			return myTarget, err, nil
		} else {
			slog.Warn("failed to elevate privileges on remote target, continuing without elevated privileges", slog.String("targetHost", targetHost))
		}
	}
	return myTarget, nil, nil
}

type targetFromYAML struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	User string `yaml:"user"`
	Key  string `yaml:"key"`
	Pwd  string `yaml:"pwd"`
}

type targetsFile struct {
	Targets []targetFromYAML `yaml:"targets"`
}

// sanitizeTargetName sanitizes the target name by removing any invalid characters.
func sanitizeTargetName(targetName string) string {
	// remove any invalid characters from the target name
	// this is needed for the report file names
	// we only allow alphanumeric characters, underscores, periods, and dashes
	// everything else is replaced with an underscore
	sanitizedTargetName := strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' {
			return r
		}
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r
		}
		if r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, targetName)
	return sanitizedTargetName
}

// getTargetsFromFile reads a targets file and returns a list of target objects.
// It takes the path to the targets file and the local temporary directory as input.
func getTargetsFromFile(targetsFilePath string, localTempDir string) (targets []target.Target, targetErrs []error, err error) {
	var targetsFile targetsFile
	// read the file into a byte array
	yamlFile, err := os.ReadFile(targetsFilePath) // #nosec G304
	if err != nil {
		return
	}
	// parse the file contents into a targetsFile struct
	err = yaml.Unmarshal(yamlFile, &targetsFile)
	if err != nil {
		return
	}

	// create target objects from the targetFromYAML structs
	hostArchitecture, err := getHostArchitecture()
	if err != nil {
		return
	}
	targetNameUsed := make(map[string]bool)
	for _, t := range targetsFile.Targets {
		// create a target object
		// target name is not required, but if it is provided there must not be duplicate names
		var targetName string
		if t.Name != "" {
			targetName = sanitizeTargetName(t.Name)
			if targetNameUsed[targetName] {
				err = fmt.Errorf("duplicate target name (after sanitized) found in targets file: original: %s, sanitized: %s", t.Name, targetName)
				return
			}
			targetNameUsed[targetName] = true
		}
		newTarget := target.NewRemoteTarget(targetName, t.Host, t.Port, t.User, t.Key)
		newTarget.SetSshPass(t.Pwd)
		// create a sub-directory for the target in the localTempDir
		localTargetDir := path.Join(localTempDir, newTarget.GetName())
		err = os.MkdirAll(localTargetDir, 0700)
		if err != nil {
			return
		}
		// if the target has a password, extract sshpass into the target-specific temp dir and set the path
		var sshPassPath string
		if t.Pwd != "" {
			sshPassPath, err = util.ExtractResource(script.Resources, path.Join("resources", hostArchitecture, "sshpass"), localTargetDir)
			if err != nil {
				return
			}
			newTarget.SetSshPassPath(sshPassPath)
		}
		// try to connect to the target
		if !newTarget.CanConnect() {
			targetErrs = append(targetErrs, fmt.Errorf("failed to connect to target host (%s)", newTarget.GetName()))
		} else {
			targetErrs = append(targetErrs, nil)
		}
		targets = append(targets, newTarget)
	}
	return
}

// getPassword prompts the user for a password and returns it as a string.
// It takes a prompt string as input and displays the prompt to the user.
// The user's input is hidden as they type, and the entered password is returned as a string.
// If an error occurs while reading the password, it is returned along with an empty string.
func getPassword(prompt string) (string, error) {
	fmt.Fprintf(os.Stderr, "\n%s: ", prompt)
	pwd, err := term.ReadPassword(0)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "\n") // newline after password
	return string(pwd), nil
}

func getHostArchitecture() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return cpus.X86Architecture, nil
	case "arm64":
		return cpus.ARMArchitecture, nil
	default:
		slog.Error("unsupported architecture", slog.String("architecture", runtime.GOARCH))
		err := fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		return "", err
	}
}

// fieldFromDfpOutput parses the output of the `df -P <dir>` command and returns the specified field value.
// example output:
//
//	Filesystem     1024-blocks     Used  Available Capacity Mounted on
//	/dev/sda2       1858388360 17247372 1747419536       1% /
//
// Returns the value of the specified field from the second line of the output.
func fieldFromDfpOutput(dfOutput string, fieldName string) (string, error) {
	lines := strings.Split(dfOutput, "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected output from df command: %s", dfOutput)
	}
	// find the field index from the header
	headerFields := strings.Fields(lines[0])
	fieldIndex := -1
	for i, field := range headerFields {
		if field == fieldName {
			fieldIndex = i
			break
		}
	}
	if fieldIndex == -1 {
		return "", fmt.Errorf("field %s not found in df output", fieldName)
	}
	// get the value from the second line (the actual data)
	dfFields := strings.Fields(lines[1])
	if len(dfFields) <= fieldIndex {
		return "", fmt.Errorf("unexpected output format from df command: %s", dfOutput)
	}
	return dfFields[fieldIndex], nil
}

type mountRecord struct {
	fileSystem string
	mountPoint string
	typeName   string
	options    []string
}

// parseMountOutput parses the output of the `mount` command and returns a slice of mountRecord structs.
// e.g., "sysfs on /sys type sysfs (rw,nosuid,nodev,noexec,relatime)"
func parseMountOutput(mountOutput string) ([]mountRecord, error) {
	var mounts []mountRecord
	for line := range strings.SplitSeq(mountOutput, "\n") {
		if line == "" {
			continue
		}
		re := regexp.MustCompile(`^([^ ]+) on ([^ ]+) type ([^ ]+) \((.*)\)$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) != 5 {
			return nil, fmt.Errorf("unexpected output format from mount command: %s", line)
		}
		// create a mountRecord struct and append it to the slice
		mount := mountRecord{
			fileSystem: matches[1],
			mountPoint: matches[2],
			typeName:   matches[3],
			options:    strings.Split(matches[4], ","),
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

// isDirNoExec checks if the target directory is on a file system that is mounted with noexec.
func isDirNoExec(t target.Target, dir string) (bool, error) {
	dfCmd := exec.Command("df", "-P", dir)
	dfOutput, _, _, err := t.RunCommand(dfCmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to run df command: %w", err)
		return false, err
	}
	filesystem, err := fieldFromDfpOutput(dfOutput, "Filesystem")
	if err != nil {
		return false, err
	}
	mountedOn, err := fieldFromDfpOutput(dfOutput, "Mounted")
	if err != nil {
		return false, err
	}
	mountCmd := exec.Command("mount")
	mountOutput, _, _, err := t.RunCommand(mountCmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to run mount command: %w", err)
		return false, err
	}
	mounts, err := parseMountOutput(mountOutput)
	if err != nil {
		return false, err
	}
	if len(mounts) == 0 {
		return false, fmt.Errorf("no mount records found")
	}
	// Check if the filesystem is mounted with noexec
	foundFilesystem := false
	foundMountPoint := false
	for _, mount := range mounts {
		if mount.fileSystem == filesystem {
			foundFilesystem = true
		}
		if mount.mountPoint == mountedOn {
			foundMountPoint = true
		}
		if mount.fileSystem == filesystem && mount.mountPoint == mountedOn {
			return slices.Contains(mount.options, "noexec"), nil
		}
	}
	if foundMountPoint {
		return false, fmt.Errorf("mount point %s is found but filesystem %s is not found in mount records", mountedOn, filesystem)
	}
	if foundFilesystem {
		return false, fmt.Errorf("filesystem %s is found but mount point %s is not found in mount records", filesystem, mountedOn)
	}
	return false, fmt.Errorf("filesystem %s and mount point %s are not found in mount records", filesystem, mountedOn)
}

func GetTargetArchitecture(t target.Target) (string, error) {
	return t.GetArchitecture()
}

func GetTargetVendor(t target.Target) (string, error) {
	vendor := t.GetVendor()
	if vendor == "" {
		cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^Vendor ID:\" | awk '{print $NF}'")
		var err error
		vendor, _, _, err = t.RunCommand(cmd, 0, true)
		if err != nil {
			return "", fmt.Errorf("failed to get target CPU vendor: %v", err)
		}
		vendor = strings.TrimSpace(vendor)
		t.SetVendor(vendor)
	}
	return vendor, nil
}

func GetTargetFamily(t target.Target) (string, error) {
	family := t.GetFamily()
	if family == "" {
		cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^CPU family:\" | awk '{print $NF}'")
		var err error
		family, _, _, err = t.RunCommand(cmd, 0, true)
		if err != nil {
			return "", fmt.Errorf("failed to get target CPU family: %v", err)
		}
		family = strings.TrimSpace(family)
		t.SetFamily(family)
	}
	return family, nil
}

func GetTargetModel(t target.Target) (string, error) {
	model := t.GetModel()
	if model == "" {
		cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^Model:\" | awk '{print $NF}'")
		var err error
		model, _, _, err = t.RunCommand(cmd, 0, true)
		if err != nil {
			return "", fmt.Errorf("failed to get target CPU model: %v", err)
		}
		model = strings.TrimSpace(model)
		t.SetModel(model)
	}
	return model, nil
}

func GetTargetStepping(t target.Target) (string, error) {
	stepping := t.GetStepping()
	if stepping == "" {
		cmd := exec.Command("bash", "-c", "lscpu | grep -i \"^Stepping:\" | awk '{print $NF}'")
		var err error
		stepping, _, _, err = t.RunCommand(cmd, 0, true)
		if err != nil {
			return "", fmt.Errorf("failed to get target CPU stepping: %v", err)
		}
		stepping = strings.TrimSpace(stepping)
		t.SetStepping(stepping)
	}
	return stepping, nil
}

func GetTargetCapid4(t target.Target, localTempDir string, noRoot bool) (string, error) {
	capid4 := t.GetCapid4()
	if capid4 == "" {
		getScript := script.GetScriptByName(script.LspciBitsScriptName)
		scriptOutput, err := script.RunScript(t, getScript, localTempDir) // don't call common.RunScript, otherwise infinite loop
		if err != nil {
			return "", fmt.Errorf("failed to run lspci bits script: %v", err)
		}
		capid4 = strings.TrimSpace(scriptOutput.Stdout)
		t.SetCapid4(capid4)
	}
	return capid4, nil
}

func GetTargetDevices(t target.Target, localTempDir string, noRoot bool) (string, error) {
	devices := t.GetDevices()
	if devices == "" {
		getScript := script.GetScriptByName(script.LspciDevicesScriptName)
		scriptOutput, err := script.RunScript(t, getScript, localTempDir) // don't call common.RunScript, otherwise infinite loop
		if err != nil {
			return "", fmt.Errorf("failed to run lspci devices script: %v", err)
		}
		devices = strings.TrimSpace(scriptOutput.Stdout)
		t.SetDevices(devices)
	}
	return devices, nil
}

func GetTargetImplementer(t target.Target, localTempDir string, noRoot bool) (string, error) {
	implementer := t.GetImplementer()
	if implementer == "" {
		getScript := script.GetScriptByName(script.ArmImplementerScriptName)
		scriptOutput, err := script.RunScript(t, getScript, localTempDir)
		if err != nil {
			return "", fmt.Errorf("failed to run implementer retrieval script: %v", err)
		}
		implementer = strings.TrimSpace(scriptOutput.Stdout)
		t.SetImplementer(implementer)
	}
	return implementer, nil
}

func GetTargetPart(t target.Target, localTempDir string, noRoot bool) (string, error) {
	part := t.GetPart()
	if part == "" {
		getScript := script.GetScriptByName(script.ArmPartScriptName)
		scriptOutput, err := script.RunScript(t, getScript, localTempDir)
		if err != nil {
			return "", fmt.Errorf("failed to run part retrieval script: %v", err)
		}
		part = strings.TrimSpace(scriptOutput.Stdout)
		t.SetPart(part)
	}
	return part, nil
}

func GetTargetDmidecodePart(t target.Target, localTempDir string, noRoot bool) (string, error) {
	dmidecodePart := t.GetDmidecodePart()
	if dmidecodePart == "" {
		getScript := script.GetScriptByName(script.ArmDmidecodePartScriptName)
		scriptOutput, err := script.RunScript(t, getScript, localTempDir) // don't call common.RunScript, otherwise infinite loop
		if err != nil {
			return "", fmt.Errorf("failed to run dmidecode part number script: %v", err)
		}
		dmidecodePart = strings.TrimSpace(scriptOutput.Stdout)
		t.SetDmidecodePart(dmidecodePart)
	}
	return dmidecodePart, nil
}

func GetTargetMicroArchitecture(t target.Target, localTempDir string, noRoot bool) (string, error) {
	uarch := t.GetMicroarchitecture()
	if uarch == "" {
		arch, err := GetTargetArchitecture(t)
		if err != nil {
			return "", err
		}
		switch arch {
		case cpus.X86Architecture:
			uarch, err = GetX86TargetMicroarchitecture(t, localTempDir, noRoot)
		case cpus.ARMArchitecture:
			uarch, err = GetARMTargetMicroarchitecture(t, localTempDir, noRoot)
		default:
			uarch, err = "", fmt.Errorf("unsupported architecture: %s", arch)
		}
		if err != nil {
			return "", err
		}
		t.SetMicroarchitecture(uarch)
	}
	return uarch, nil
}

func GetX86TargetMicroarchitecture(t target.Target, localTempDir string, noRoot bool) (string, error) {
	family, err := GetTargetFamily(t)
	if err != nil {
		return "", err
	}
	model, err := GetTargetModel(t)
	if err != nil {
		return "", err
	}
	stepping, err := GetTargetStepping(t)
	if err != nil {
		return "", err
	}
	capid4, err := GetTargetCapid4(t, localTempDir, noRoot)
	if err != nil {
		slog.Warn("failed to read lspci bits to get capid4 for microarchitecture identification", slog.String("error", err.Error()))
		// continue
	}
	devices, err := GetTargetDevices(t, localTempDir, noRoot)
	if err != nil {
		slog.Warn("failed to read lspci devices for microarchitecture identification", slog.String("error", err.Error()))
		// continue
	}
	cpu, err := cpus.GetCPU(cpus.NewX86Identifier(family, model, stepping, capid4, devices))
	if err != nil {
		return "", err
	}
	return cpu.MicroArchitecture, nil
}

func GetARMTargetMicroarchitecture(t target.Target, localTempDir string, noRoot bool) (string, error) {
	implementer, err := GetTargetImplementer(t, localTempDir, noRoot)
	if err != nil {
		return "", err
	}
	part, err := GetTargetPart(t, localTempDir, noRoot)
	if err != nil {
		return "", err
	}
	dmidecodePart, err := GetTargetDmidecodePart(t, localTempDir, noRoot)
	if err != nil {
		return "", err
	}
	cpu, err := cpus.GetCPU(cpus.NewARMIdentifier(implementer, part, dmidecodePart))
	if err != nil {
		return "", err
	}
	return cpu.MicroArchitecture, nil
}

func ScriptSupportedOnTarget(t target.Target, scriptDef script.ScriptDefinition, localTempDir string, noRoot bool) (bool, error) {
	if len(scriptDef.Architectures) > 0 {
		arch, err := GetTargetArchitecture(t)
		if err != nil {
			return false, err
		}
		if !slices.Contains(scriptDef.Architectures, arch) {
			slog.Info("script not supported on target architecture", slog.String("script", scriptDef.Name), slog.String("target", t.GetName()), slog.String("architecture", arch))
			return false, nil
		}
	}
	if len(scriptDef.Vendors) > 0 {
		vendor, err := GetTargetVendor(t)
		if err != nil {
			return false, err
		}
		if !slices.Contains(scriptDef.Vendors, vendor) {
			slog.Info("script not supported on target CPU vendor", slog.String("script", scriptDef.Name), slog.String("target", t.GetName()), slog.String("vendor", vendor))
			return false, nil
		}
	}
	if len(scriptDef.MicroArchitectures) > 0 {
		uarch, err := GetTargetMicroArchitecture(t, localTempDir, noRoot)
		if err != nil {
			return false, err
		}
		shortUarch := strings.Split(uarch, "_")[0]     // handle EMR_XCC, etc.
		shortUarch = strings.Split(shortUarch, "-")[0] // handle GNR-D
		shortUarch = strings.Split(shortUarch, " ")[0] // handle Turin (Zen 5)
		if !slices.Contains(scriptDef.MicroArchitectures, uarch) && !slices.Contains(scriptDef.MicroArchitectures, shortUarch) {
			slog.Info("script not supported on target CPU microarchitecture", slog.String("script", scriptDef.Name), slog.String("target", t.GetName()), slog.String("microarchitecture", uarch))
			return false, nil
		}
	}
	return true, nil
}

func FilterScriptsForTarget(t target.Target, scriptDefs []script.ScriptDefinition, localTempDir string, noRoot bool) (supportedScripts []script.ScriptDefinition, err error) {
	for _, scriptDef := range scriptDefs {
		supported, err := ScriptSupportedOnTarget(t, scriptDef, localTempDir, noRoot)
		if err != nil {
			slog.Warn("failed to determine if script is compatible with target", slog.String("script", scriptDef.Name), slog.String("target", t.GetName()))
			continue
		}
		if supported {
			supportedScripts = append(supportedScripts, scriptDef)
		}
	}
	return
}

// Create wrappers around script.RunScript* that first check if the scripts are compatible with the target

func RunScript(t target.Target, s script.ScriptDefinition, localTempDir string, noRoot bool) (script.ScriptOutput, error) {
	supported, err := ScriptSupportedOnTarget(t, s, localTempDir, noRoot)
	if err != nil {
		return script.ScriptOutput{}, fmt.Errorf("failed to check if script is supported on target %v", err)
	}
	if !supported {
		return script.ScriptOutput{}, fmt.Errorf("script %s not supported on target %s", s.Name, t.GetName())
	}
	return script.RunScript(t, s, localTempDir)
}

func RunScripts(t target.Target, s []script.ScriptDefinition, ignoreScriptErrors bool, localTempDir string, statusUpdate progress.MultiSpinnerUpdateFunc, collectingStatusMsg string, noRoot bool) (map[string]script.ScriptOutput, error) {
	supportedScripts, err := FilterScriptsForTarget(t, s, localTempDir, noRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to check if scripts are supported on target %v", err)
	}
	if len(supportedScripts) == 0 {
		return nil, fmt.Errorf("zero scripts are supported on target %s", t.GetName())
	}
	return script.RunScripts(t, supportedScripts, ignoreScriptErrors, localTempDir, statusUpdate, collectingStatusMsg)
}

func RunScriptStream(t target.Target, s script.ScriptDefinition, localTempDir string, stdoutChannel chan []byte, stderrChannel chan []byte, exitcodeChannel chan int, errorChannel chan error, cmdChannel chan *exec.Cmd, noRoot bool) {
	supported, err := ScriptSupportedOnTarget(t, s, localTempDir, noRoot)
	if err != nil {
		errorChannel <- fmt.Errorf("failed to check if script is supported on target %v", err)
		return
	}
	if !supported {
		errorChannel <- fmt.Errorf("script %s not supported on target %s", s.Name, t.GetName())
		return
	}
	script.RunScriptStream(t, s, localTempDir, stdoutChannel, stderrChannel, exitcodeChannel, errorChannel, cmdChannel)
}

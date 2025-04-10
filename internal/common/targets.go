package common

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"regexp"
	"runtime"
	"strconv"
	"strings"

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
		re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)$`)
		if !re.MatchString(flagTargetUser) {
			return fmt.Errorf("user name %s contains invalid characters", flagTargetUser)
		}
	}
	// confirm that host is a valid host name or IP address
	if flagTargetHost != "" {
		re := regexp.MustCompile(`^([a-zA-Z0-9.-]+)$`)
		if !re.MatchString(flagTargetHost) {
			return fmt.Errorf("host name %s is not a valid host name or IP address", flagTargetHost)
		}
	}
	return nil
}

// GetTargets retrieves the list of targets based on the provided command and parameters. It creates
// a temporary directory for each target and returns a slice of target.Target objects.
func GetTargets(cmd *cobra.Command, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (targets []target.Target, targetErrs []error, err error) {
	targetTempDirRoot := cmd.Parent().PersistentFlags().Lookup("tempdir").Value.String()
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
		if targetErrs[targetIdx] != nil {
			continue
		}
		_, err := myTarget.CreateTempDirectory(targetTempDirRoot)
		if err != nil {
			targetErrs[targetIdx] = fmt.Errorf("failed to create temp directory on target")
			slog.Error(targetErrs[targetIdx].Error(), slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			continue
		}
		// confirm that the temp directory was not created on a file system mounted with noexec
		noExec, err := isNoExec(myTarget, myTarget.GetTempDirectory())
		if err != nil {
			// log the error but don't reject the target just in case our check is wrong
			slog.Error("failed to check if temp directory is mounted on 'noexec' file system", slog.String("target", myTarget.GetName()), slog.String("error", err.Error()))
			continue
		}
		if noExec {
			targetErrs[targetIdx] = fmt.Errorf("target's temp directory must not be on a file system mounted with the 'noexec' option, override the default with --tempdir")
			slog.Error(targetErrs[targetIdx].Error(), slog.String("target", myTarget.GetName()))
			continue
		}
	}
	return
}

// isNoExec checks if the temporary directory is on a file system that is mounted with noexec.
func isNoExec(t target.Target, tempDir string) (bool, error) {
	dfCmd := exec.Command("df", "-P", tempDir)
	dfOutput, _, _, err := t.RunCommand(dfCmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to run df command: %w", err)
		return false, err
	}
	mountCmd := exec.Command("mount")
	mountOutput, _, _, err := t.RunCommand(mountCmd, 0, true)
	if err != nil {
		err = fmt.Errorf("failed to run mount command: %w", err)
		return false, err
	}
	// Parse the output of `df` to extract the device name
	lines := strings.Split(dfOutput, "\n")
	if len(lines) < 2 {
		return false, fmt.Errorf("unexpected output from df command: %s", dfOutput)
	}
	dfFields := strings.Fields(lines[1]) // Second line contains the device info
	if len(dfFields) < 6 {
		return false, fmt.Errorf("unexpected output format from df command: %s", dfOutput)
	}
	filesystem := dfFields[0]
	mountedOn := dfFields[5]
	// Search for the device in the mount output and check for "noexec"
	var found bool
	for line := range strings.SplitSeq(mountOutput, "\n") {
		mountFields := strings.Fields(line)
		if len(mountFields) < 6 {
			continue // Skip lines that don't have enough fields
		}
		device := mountFields[0]
		mountPoint := mountFields[2]
		mountOptions := strings.Join(mountFields[5:], " ")
		if device == filesystem && mountPoint == mountedOn {
			found = true
			if strings.Contains(mountOptions, "noexec") {
				return true, nil // Found "noexec" for the device
			} else {
				break
			}
		}
	}
	if !found {
		return false, fmt.Errorf("device %s not found in mount output", filesystem)
	}
	return false, nil // "noexec" not found
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
	err := os.MkdirAll(localTargetDir, 0755)
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
	slog.Info("Creating remote target", slog.String("targetHost", targetHost), slog.String("targetPort", targetPort), slog.String("targetUser", targetUser))
	myTarget := target.NewRemoteTarget(targetHost, targetHost, targetPort, targetUser, targetKey)
	// create a sub-directory for the target in the localTempDir
	localTargetDir := path.Join(localTempDir, myTarget.GetName())
	err := os.MkdirAll(localTargetDir, 0755)
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

// getTargetsFromFile reads a targets file and returns a list of target objects.
// It takes the path to the targets file and the local temporary directory as input.
func getTargetsFromFile(targetsFilePath string, localTempDir string) (targets []target.Target, targetErrs []error, err error) {
	var targetsFile targetsFile
	// read the file into a byte array
	yamlFile, err := os.ReadFile(targetsFilePath)
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
	for _, t := range targetsFile.Targets {
		// create a sub-directory for each target in the localTempDir
		localTargetDir := path.Join(localTempDir, t.Name)
		err = os.MkdirAll(localTargetDir, 0755)
		if err != nil {
			return
		}
		// extract sshpass resource if password is provided
		var sshPassPath string
		if t.Pwd != "" {
			sshPassPath, err = util.ExtractResource(script.Resources, path.Join("resources", hostArchitecture, "sshpass"), localTargetDir)
			if err != nil {
				return
			}
		}
		// create a target object
		newTarget := target.NewRemoteTarget(t.Name, t.Host, t.Port, t.User, t.Key)
		newTarget.SetSshPassPath(sshPassPath)
		newTarget.SetSshPass(t.Pwd)
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
	if runtime.GOARCH == "amd64" {
		return "x86_64", nil
	} else if runtime.GOARCH == "arm64" {
		return "aarch64", nil
	} else {
		slog.Error("unsupported architecture", slog.String("architecture", runtime.GOARCH))
		err := fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		return "", err
	}
}

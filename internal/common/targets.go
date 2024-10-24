package common

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path"
	"perfspect/internal/script"
	"perfspect/internal/target"
	"perfspect/internal/util"
	"runtime"

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
	flagTargetTempDir string
)

// target flag names
const (
	flagTargetsFileName   = "targets"
	flagTargetHostName    = "target"
	flagTargetPortName    = "port"
	flagTargetUserName    = "user"
	flagTargetKeyName     = "key"
	FlagTargetTempDirName = "targettemp"
)

var targetFlags = []Flag{
	{Name: flagTargetHostName, Help: "host name or IP address of remote target"},
	{Name: flagTargetPortName, Help: "port for SSH to remote target"},
	{Name: flagTargetUserName, Help: "user name for SSH to remote target"},
	{Name: flagTargetKeyName, Help: "private key file for SSH to remote target"},
	{Name: flagTargetsFileName, Help: "file with remote target(s) connection details. See targets.yaml for format."},
	{Name: FlagTargetTempDirName, Help: "directory to use on remote target for temporary files"},
}

func AddTargetFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagTargetHost, flagTargetHostName, "", targetFlags[0].Help)
	cmd.Flags().StringVar(&flagTargetPort, flagTargetPortName, "", targetFlags[1].Help)
	cmd.Flags().StringVar(&flagTargetUser, flagTargetUserName, "", targetFlags[2].Help)
	cmd.Flags().StringVar(&flagTargetKeyFile, flagTargetKeyName, "", targetFlags[3].Help)
	cmd.Flags().StringVar(&flagTargetsFile, flagTargetsFileName, "", targetFlags[4].Help)
	cmd.Flags().StringVar(&flagTargetTempDir, FlagTargetTempDirName, "", targetFlags[5].Help)

	cmd.MarkFlagsMutuallyExclusive(flagTargetHostName, flagTargetsFileName)
}

func GetTargetFlagGroup() FlagGroup {
	return FlagGroup{
		GroupName: "Remote Target Options",
		Flags:     targetFlags,
	}
}

// GetTargets retrieves the list of targets based on the provided command and parameters.
// If a targets file is specified, it reads the targets from the file.
// Otherwise, it retrieves a single target using the getTarget function.
// The function returns a slice of target.Target and an error if any.
func GetTargets(cmd *cobra.Command, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) ([]target.Target, error) {
	flagTargetsFile, _ := cmd.Flags().GetString(flagTargetsFileName)
	if flagTargetsFile != "" {
		return getTargetsFromFile(flagTargetsFile, localTempDir)
	}
	myTarget, err := getTarget(cmd, needsElevatedPrivileges, failIfCantElevate, localTempDir)
	if err != nil {
		return nil, err
	}
	return []target.Target{myTarget}, nil
}

// getTarget returns a target.Target object representing the target host and associated details.
// The function takes the following parameters:
// - cmd: A pointer to the cobra.Command object representing the command.
// - needsElevatedPriviliges: A boolean indicating whether elevated privileges are required.
// - failIfCantElevate: A boolean indicating whether to fail if elevated privileges can't be obtained.
// - localTempDir: A string representing the local temporary directory.
// The function returns the following values:
// - myTarget: A target.Target object representing the target host and associated details.
// - err: An error object indicating any error that occurred during the retrieval process.
func getTarget(cmd *cobra.Command, needsElevatedPrivileges bool, failIfCantElevate bool, localTempDir string) (target.Target, error) {
	targetHost, _ := cmd.Flags().GetString(flagTargetHostName)
	targetPort, _ := cmd.Flags().GetString(flagTargetPortName)
	targetUser, _ := cmd.Flags().GetString(flagTargetUserName)
	targetKey, _ := cmd.Flags().GetString(flagTargetKeyName)
	if targetHost != "" {
		myTarget := target.NewRemoteTarget(targetHost, targetHost, targetPort, targetUser, targetKey)
		if !myTarget.CanConnect() {
			if targetKey == "" && targetUser != "" {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					err := fmt.Errorf("can not prompt for SSH password because STDIN isn't coming from a terminal")
					slog.Error(err.Error())
					return myTarget, err
				} else {
					slog.Info("Prompting for SSH password.", slog.String("targetHost", targetHost), slog.String("targetUser", targetUser))
					sshPwd, err := getPassword(fmt.Sprintf("%s@%s's password", targetUser, targetHost))
					if err != nil {
						return nil, err
					}
					var hostArchitecture string
					hostArchitecture, err = getHostArchitecture()
					if err != nil {
						return nil, err
					}
					sshPassPath, err := util.ExtractResource(script.Resources, path.Join("resources", hostArchitecture, "sshpass"), localTempDir)
					if err != nil {
						return nil, err
					}
					myTarget.SetSshPassPath(sshPassPath)
					myTarget.SetSshPass(sshPwd)
					// if still can't connect, return error
					if !myTarget.CanConnect() {
						err = fmt.Errorf("failed to connect to target host (%s) using provided ssh user (%s) and password (****)", targetHost, targetUser)
						return nil, err
					}
				}
			} else {
				err := fmt.Errorf("failed to connect to target (%s) using provided target arguments", targetHost)
				return nil, err
			}
		}
		if needsElevatedPrivileges && !myTarget.CanElevatePrivileges() {
			if failIfCantElevate {
				err := fmt.Errorf("failed to elevate privileges on remote target")
				return nil, err
			} else {
				slog.Warn("failed to elevate privileges on remote target, continuing without elevated privileges", slog.String("targetHost", targetHost))
			}
		}
		return myTarget, nil
	}
	// local target
	myTarget := target.NewLocalTarget()
	if needsElevatedPrivileges && !myTarget.CanElevatePrivileges() {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			slog.Warn("can not prompt for sudo password because STDIN isn't coming from a terminal")
			if failIfCantElevate {
				err := fmt.Errorf("failed to elevate privileges on local target")
				return nil, err
			} else {
				slog.Warn("continuing without elevated privileges")
			}
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: some operations cannot be performed without elevated privileges.\n")
			currentUser, err := user.Current()
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "For complete functionality, please provide your password at the prompt.\n")
			slog.Info("prompting for sudo password")
			prompt := fmt.Sprintf("[sudo] password for %s", currentUser.Username)
			var sudoPwd string
			sudoPwd, err = getPassword(prompt)
			if err != nil {
				return nil, err
			}
			myTarget.SetSudo(sudoPwd)
			if !myTarget.CanElevatePrivileges() {
				if failIfCantElevate {
					err := fmt.Errorf("failed to elevate privileges on local target")
					return nil, err
				} else {
					slog.Warn("failed to elevate privileges on local target, continuing without elevated privileges")
					fmt.Fprintf(os.Stderr, "WARNING: Not able to establish elevated privileges with provided password.\n")
					fmt.Fprintf(os.Stderr, "Continuing with regular user privileges. Some operations will not be performed.\n")
				}
			}
		}
	}
	return myTarget, nil
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
func getTargetsFromFile(targetsFilePath string, localTempDir string) (targets []target.Target, err error) {
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

	// if any of the targets require a password, extract sshpass from resources
	needsSshPass := false
	for _, t := range targetsFile.Targets {
		if t.Pwd != "" {
			needsSshPass = true
			break
		}
	}
	var sshPassPath string
	if needsSshPass {
		var hostArchitecture string
		hostArchitecture, err = getHostArchitecture()
		if err != nil {
			return
		}
		sshPassPath, err = util.ExtractResource(script.Resources, path.Join("resources", hostArchitecture, "sshpass"), localTempDir)
		if err != nil {
			return
		}
	}
	// create target objects from the targetFromYAML structs
	for _, t := range targetsFile.Targets {
		newTarget := target.NewRemoteTarget(t.Name, t.Host, t.Port, t.User, t.Key)
		newTarget.SetSshPassPath(sshPassPath)
		newTarget.SetSshPass(t.Pwd)
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

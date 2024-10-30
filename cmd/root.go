// Package cmd provides the command line interface for the application.
package cmd

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"perfspect/cmd/config"
	"perfspect/cmd/flame"
	"perfspect/cmd/metrics"
	"perfspect/cmd/report"
	"perfspect/cmd/telemetry"
	"perfspect/internal/common"
	"perfspect/internal/util"

	"github.com/spf13/cobra"
)

var gLogFile *os.File
var gVersion = "9.9.9" // overwritten by ldflags in Makefile, set to high number here to avoid update prompt while debugging

const (
	// LongAppName is the name of the application
	LongAppName = "PerfSpect"
)

var examples = []string{
	fmt.Sprintf("  Generate a configuration report:                             $ %s report", common.AppName),
	fmt.Sprintf("  Monitor micro-architectural metrics:                         $ %s metrics", common.AppName),
	fmt.Sprintf("  Generate a configuration report on a remote target:          $ %s report --target 192.168.1.2 --user elaine --key ~/.ssh/id_rsa", common.AppName),
	fmt.Sprintf("  Generate configuration reports for multiple remote targets:  $ %s report --targets ./targets.yaml", common.AppName),
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:                common.AppName,
	Short:              common.AppName,
	Long:               fmt.Sprintf(`%s (%s) is a multi-function utility for performance engineers analyzing software running on Intel Xeon platforms.`, LongAppName, common.AppName),
	Example:            strings.Join(examples, "\n"),
	PersistentPreRunE:  initializeApplication, // will only be run if command has a 'Run' function
	PersistentPostRunE: terminateApplication,  // ...
	Version:            gVersion,
}

var (
	// logging
	flagDebug  bool
	flagSyslog bool
	// output
	flagOutputDir     string
	flagTempDir       string
	flagNoCheckUpdate bool
)

const (
	flagDebugName         = "debug"
	flagSyslogName        = "syslog"
	flagOutputDirName     = "output"
	flagTempDirName       = "tempdir"
	flagNoCheckUpdateName = "noupdate"
)

func init() {
	rootCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command] [flags]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}
`)
	rootCmd.SetHelpCommand(&cobra.Command{}) // block the help command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddGroup([]*cobra.Group{{ID: "primary", Title: "Commands:"}}...)
	rootCmd.AddCommand(report.Cmd)
	rootCmd.AddCommand(metrics.Cmd)
	rootCmd.AddCommand(telemetry.Cmd)
	rootCmd.AddCommand(flame.Cmd)
	rootCmd.AddCommand(config.Cmd)
	if onIntelNetwork() {
		rootCmd.AddGroup([]*cobra.Group{{ID: "other", Title: "Other Commands:"}}...)
		rootCmd.AddCommand(updateCmd)
	}
	// Global (persistent) flags
	rootCmd.PersistentFlags().BoolVar(&flagDebug, flagDebugName, false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&flagSyslog, flagSyslogName, false, "log to syslog")
	rootCmd.PersistentFlags().StringVar(&flagOutputDir, flagOutputDirName, "", "override the output directory")
	rootCmd.PersistentFlags().StringVar(&flagTempDir, flagTempDirName, "", "override the local temp directory")
	rootCmd.PersistentFlags().BoolVar(&flagNoCheckUpdate, flagNoCheckUpdateName, false, "skip application update check")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false
	cobra.EnableCaseInsensitive = true
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initializeApplication(cmd *cobra.Command, args []string) error {
	var err error
	// verify requested output directory exists or create an output directory
	var outputDir string
	if flagOutputDir != "" {
		var err error
		outputDir, err = util.AbsPath(flagOutputDir)
		if err != nil {
			fmt.Printf("Error: failed to expand output dir %v\n", err)
			os.Exit(1)
		}
		exists, err := util.DirectoryExists(outputDir)
		if err != nil {
			fmt.Printf("Error: failed to determine if output dir exists: %v\n", err)
			os.Exit(1)
		}
		if !exists {
			fmt.Printf("Error: requested output dir, %s, does not exist\n", outputDir)
			os.Exit(1)
		}
	} else {
		// set output dir path to app name + timestamp (dont' create the directory)
		outputDirName := common.AppName + "_" + time.Now().Local().Format("2006-01-02_15-04-05")
		var err error
		// outputDir will be in current working directory
		outputDir, err = util.AbsPath(outputDirName)
		if err != nil {
			fmt.Printf("Error: failed to expand output dir %v\n", err)
			os.Exit(1)
		}
	}
	// open log file in current directory
	gLogFile, err = os.OpenFile(common.AppName+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error: failed to open log file: %v\n", err)
		os.Exit(1)
	}
	var logLevel slog.Leveler
	var logSource bool
	if flagDebug {
		logLevel = slog.LevelDebug
		logSource = true
	} else {
		logLevel = slog.LevelInfo
		logSource = false
	}
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: logSource,
	}
	logger := slog.New(slog.NewTextHandler(gLogFile, opts))
	slog.SetDefault(logger)
	slog.Info("Starting up", slog.String("app", common.AppName), slog.String("version", gVersion), slog.Int("PID", os.Getpid()), slog.String("arguments", strings.Join(os.Args, " ")))
	// verify requested local temp dir exists
	var localTempDir string
	if flagTempDir != "" {
		localTempDir, err = util.AbsPath(flagTempDir)
		if err != nil {
			fmt.Printf("Error: failed to expand temp dir path: %v\n", err)
			os.Exit(1)
		}
		exists, err := util.DirectoryExists(localTempDir)
		if err != nil {
			fmt.Printf("Error: failed to determine if temp dir path exists: %v\n", err)
			os.Exit(1)
		}
		if !exists {
			fmt.Printf("Error: requested temp dir, %s, does not exist\n", localTempDir)
			os.Exit(1)
		}
	} else {
		localTempDir = os.TempDir()
	}
	applicationTempDir, err := os.MkdirTemp(localTempDir, fmt.Sprintf("%s.tmp.", common.AppName))
	if err != nil {
		fmt.Printf("Error: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	cmd.SetContext(
		context.WithValue(
			context.Background(),
			common.AppContext{},
			common.AppContext{
				OutputDir: outputDir,
				TempDir:   applicationTempDir,
				Version:   gVersion},
		),
	)
	// check for updates unless the user has disabled this feature or is not on the Intel network or is running the update command
	if !flagNoCheckUpdate && onIntelNetwork() && cmd.Name() != "update" {
		// catch signals to allow for graceful shutdown
		sigChannel := make(chan os.Signal, 1)
		signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigChannel
			slog.Info("received signal", slog.String("signal", sig.String()))
			terminateApplication(cmd, args)
			fmt.Println()
			os.Exit(1)
		}()
		defer signal.Stop(sigChannel)
		slog.Info("Checking for updates")
		updateAvailable, latestManifest, err := checkForUpdates(gVersion)
		if err != nil {
			slog.Error(err.Error())
		} else if updateAvailable {
			fmt.Fprintf(os.Stderr, "A new version (%s) of %s is available!\nPlease run '%s update' to update to the latest version.\n\n", latestManifest.Version, common.AppName, common.AppName)
		} else {
			slog.Info("No updates available")
		}
	}
	return nil
}

func terminateApplication(cmd *cobra.Command, args []string) error {
	appContext := cmd.Context().Value(common.AppContext{}).(common.AppContext)

	// clean up temp directory
	if appContext.TempDir != "" {
		err := os.RemoveAll(appContext.TempDir)
		if err != nil {
			slog.Error("error cleaning up temp directory", slog.String("tempDir", appContext.TempDir), slog.String("error", err.Error()))
		}
	}

	slog.Info("Shutting down", slog.String("app", common.AppName), slog.String("version", gVersion), slog.Int("PID", os.Getpid()), slog.String("arguments", strings.Join(os.Args, " ")))
	if gLogFile != nil {
		gLogFile.Close()
	}
	return nil
}

// onIntelNetwork checks if the host is on the Intel network
func onIntelNetwork() bool {
	// If we can't lookup the Intel autoproxy domain then we aren't on the Intel
	// network
	_, err := net.LookupHost("wpad.intel.com")
	return err == nil
}

func checkForUpdates(version string) (bool, manifest, error) {
	latestManifest, err := getLatestManifest()
	if err != nil {
		return false, latestManifest, err
	}
	slog.Debug("Latest version", slog.String("version", latestManifest.Version))
	slog.Debug("Current version", slog.String("version", version))
	result, err := util.CompareVersions(latestManifest.Version, version)
	if err != nil {
		return false, latestManifest, err
	}
	return result == 1, latestManifest, nil
}

var updateCmd = &cobra.Command{
	GroupID: "other",
	Use:     "update",
	Short:   "Update the application",
	RunE: func(cmd *cobra.Command, args []string) error {
		appContext := cmd.Context().Value(common.AppContext{}).(common.AppContext)
		localTempDir := appContext.TempDir
		updateAvailable, latestManifest, err := checkForUpdates(gVersion)
		if err != nil {
			slog.Error("Failed to check for updates", slog.String("error", err.Error()))
			fmt.Printf("Error: update check failed: %v\n", err)
			return err
		} else if updateAvailable {
			fmt.Printf("Updating %s to version %s...\n", common.AppName, latestManifest.Version)
			err = updateApp(latestManifest, localTempDir)
			if err != nil {
				slog.Error("Failed to update application", slog.String("error", err.Error()))
				fmt.Printf("Error: failed to update application: %v\n", err)
				return err
			}
		} else {
			slog.Info("No updates available")
			fmt.Printf("No updates available for %s.\n", common.AppName)
		}
		return nil
	},
}

func updateApp(latestManifest manifest, localTempDir string) error {
	runningAppArgs := os.Args
	runningAppPath := runningAppArgs[0]
	runningAppDir := filepath.Dir(runningAppPath)
	runningAppFile := filepath.Base(runningAppPath)

	// download the latest release
	slog.Debug("Downloading latest release")
	fileName := "perfspect" + "_" + latestManifest.Version + ".tgz"
	url := "https://af01p-fm.devtools.intel.com/artifactory/perfspectnext-fm-local/releases/latest/" + fileName
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// write the tarball to a temp file
	tarballFile, err := os.CreateTemp(localTempDir, "perfspect*.tgz")
	if err != nil {
		return err
	}
	defer tarballFile.Close()
	slog.Debug("Writing tarball to temp file", slog.String("tempFile", tarballFile.Name()))
	_, err = io.Copy(tarballFile, resp.Body)
	if err != nil {
		return err
	}
	tarballFile.Close()
	// rename the running app to ".sav"
	slog.Debug("Renaming running app")
	oldAppPath := filepath.Join(runningAppDir, runningAppFile+".sav")
	err = os.Rename(runningAppPath, oldAppPath)
	if err != nil {
		return err
	}
	// rename the targets.yaml file to ".sav" if it exists
	targetsFile := filepath.Join(runningAppDir, "targets.yaml")
	if util.Exists(targetsFile) {
		slog.Debug("Renaming targets file")
		err = os.Rename(targetsFile, targetsFile+".sav")
		if err != nil {
			return err
		}
	}
	// extract the tarball over the running app's directory
	slog.Debug("Extracting tarball")
	err = util.ExtractTGZ(tarballFile.Name(), runningAppDir, true)
	if err != nil {
		slog.Error("Error extracting downloaded tarball", slog.String("error", err.Error()))
		slog.Info("Attempting to restore old executable")
		errRestore := os.Rename(oldAppPath, runningAppPath)
		if errRestore != nil {
			slog.Error("Failed to restore old executable", slog.String("error", errRestore.Error()))
		} else {
			slog.Info("Old executable restored")
		}
		slog.Info("Attempting to restore targets file")
		if util.Exists(targetsFile + ".sav") {
			errRestore = os.Rename(targetsFile+".sav", targetsFile)
			if errRestore != nil {
				slog.Error("Failed to restore targets file", slog.String("error", errRestore.Error()))
			} else {
				slog.Info("Targets file restored")
			}
		}
		return err
	}
	// remove the downloaded tarball
	slog.Debug("Removing tarball")
	err = os.Remove(tarballFile.Name())
	if err != nil {
		return err
	}
	// replace the new targets.yaml with the saved one
	if util.Exists(targetsFile + ".sav") {
		slog.Debug("Restoring targets file")
		err = os.Rename(targetsFile+".sav", targetsFile)
		if err != nil {
			return err
		}
	}
	fmt.Println("Update completed.")
	return nil
}

type manifest struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Time    string `json:"time"`
	Commit  string `json:"commit"`
}

func getLatestManifest() (manifest, error) {
	// download manifest file from server
	url := "https://af01p-fm.devtools.intel.com/artifactory/perfspectnext-fm-local/releases/latest/manifest.json"
	resp, err := http.Get(url)
	if err != nil {
		return manifest{}, err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return manifest{}, err
	}
	// parse json content in buf
	var latestManifest manifest
	err = json.Unmarshal(buf.Bytes(), &latestManifest)
	if err != nil {
		return manifest{}, err
	}
	// return latest version
	return latestManifest, nil
}

// Package cmd provides the command line interface for the application.
package cmd

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"perfspect/cmd/config"
	"perfspect/cmd/flame"
	"perfspect/cmd/lock"
	"perfspect/cmd/metrics"
	"perfspect/cmd/report"
	"perfspect/cmd/telemetry"
	"perfspect/internal/common"
	"perfspect/internal/util"

	"github.com/spf13/cobra"
)

var gLogFile *os.File
var gVersion = "9.9.9" // overwritten by ldflags in Makefile

const (
	// LongAppName is the name of the application
	LongAppName    = "PerfSpect"
	artifactoryUrl = "https://af01p-fm.devtools.intel.com/artifactory/perfspectnext-fm-local/releases/latest/"
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
	flagDebug     bool
	flagSyslog    bool
	flagLogStdOut bool
	// output
	flagOutputDir      string
	flagTargetTempRoot string
	flagNoCheckUpdate  bool
)

const (
	flagDebugName          = "debug"
	flagSyslogName         = "syslog"
	flagLogStdOutName      = "log-stdout"
	flagOutputDirName      = "output"
	flagTargetTempRootName = "tempdir"
	flagNoCheckUpdateName  = "noupdate"
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
	rootCmd.AddCommand(lock.Cmd)
	rootCmd.AddCommand(config.Cmd)
	if onIntelNetwork() {
		rootCmd.AddGroup([]*cobra.Group{{ID: "other", Title: "Other Commands:"}}...)
		rootCmd.AddCommand(updateCmd)
	}
	// Global (persistent) flags
	rootCmd.PersistentFlags().BoolVar(&flagDebug, flagDebugName, false, "enable debug logging and retain temporary directories")
	rootCmd.PersistentFlags().BoolVar(&flagSyslog, flagSyslogName, false, "write logs to syslog instead of a file")
	rootCmd.PersistentFlags().BoolVar(&flagLogStdOut, flagLogStdOutName, false, "write logs to stdout")
	rootCmd.PersistentFlags().StringVar(&flagOutputDir, flagOutputDirName, "", "override the output directory")
	rootCmd.PersistentFlags().StringVar(&flagTargetTempRoot, flagTargetTempRootName, "", "override the temporary target directory, must exist and allow execution")
	rootCmd.PersistentFlags().BoolVar(&flagNoCheckUpdate, flagNoCheckUpdateName, false, "skip application update check")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false
	cobra.EnableCaseInsensitive = true
	err := rootCmd.Execute()
	if err != nil {
		terminateErr := terminateApplication(rootCmd, os.Args)
		if terminateErr != nil {
			slog.Error("Error terminating application", slog.String("error", terminateErr.Error()))
			fmt.Printf("Error: %v\n", terminateErr)
		}
		os.Exit(1)
	}
}

func initializeApplication(cmd *cobra.Command, args []string) error {
	timestamp := time.Now().Local().Format("2006-01-02_15-04-05") // app startup time
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
		outputDirName := common.AppName + "_" + timestamp
		var err error
		// outputDir will be in current working directory
		outputDir, err = util.AbsPath(outputDirName)
		if err != nil {
			fmt.Printf("Error: failed to expand output dir %v\n", err)
			os.Exit(1)
		}
	}
	// configure logging
	var logOpts slog.HandlerOptions
	if flagDebug {
		logOpts.Level = slog.LevelDebug
		logOpts.AddSource = true
	} else {
		logOpts.Level = slog.LevelInfo
		logOpts.AddSource = false
	}
	if flagSyslog && flagLogStdOut {
		fmt.Println("Error: both syslog handler and stdout output specified. Please pick one only.")
		os.Exit(1)
	} else if flagSyslog { // log to syslog
		handler, err := NewSyslogHandler(&logOpts)
		if err != nil {
			fmt.Printf("Error: failed to create syslog handler: %v\n", err)
			os.Exit(1)
		}
		slog.SetDefault(slog.New(handler))
	} else if flagLogStdOut {
		handler := slog.NewJSONHandler(os.Stdout, &logOpts)
		slog.SetDefault(slog.New(handler))
	} else { // log to file
		// open log file in current directory
		var err error
		gLogFile, err = os.OpenFile(common.AppName+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644) // #nosec G302
		if err != nil {
			fmt.Printf("Error: failed to open log file: %v\n", err)
			os.Exit(1)
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(gLogFile, &logOpts)))
	}
	slog.Info("Starting up", slog.String("app", common.AppName), slog.String("version", gVersion), slog.Int("PID", os.Getpid()), slog.String("arguments", strings.Join(os.Args, " ")))
	// creat local temp directory
	localTempDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("%s.tmp.", common.AppName))
	if err != nil {
		fmt.Printf("Error: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	var logFilePath string
	if gLogFile != nil {
		logFilePath = gLogFile.Name()
	}
	// set app context
	cmd.Parent().SetContext(
		context.WithValue(
			context.Background(),
			common.AppContext{},
			common.AppContext{
				Timestamp:      timestamp,
				OutputDir:      outputDir,
				LocalTempDir:   localTempDir,
				LogFilePath:    logFilePath,
				TargetTempRoot: flagTargetTempRoot,
				Version:        gVersion,
				Debug:          flagDebug},
		),
	)
	// check for updates unless the user has disabled this feature or is not on the Intel network or is running the update command
	if !flagNoCheckUpdate && onIntelNetwork() && cmd.Name() != updateCommandName {
		// catch signals to allow for graceful shutdown
		sigChannel := make(chan os.Signal, 1)
		signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigChannel
			slog.Info("received signal", slog.String("signal", sig.String()))
			err := terminateApplication(cmd, args)
			if err != nil {
				slog.Error("Error terminating application", slog.String("error", err.Error()))
			}
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

// terminateApplication cleans up the application context and closes the log file
// and removes the local temp directory if it was created
func terminateApplication(cmd *cobra.Command, args []string) error {
	var ctx context.Context
	if cmd.Parent() == nil {
		ctx = cmd.Context()
	} else {
		ctx = cmd.Parent().Context()
	}
	if ctx != nil {
		ctxValue := ctx.Value(common.AppContext{})
		if ctxValue != nil {
			if appContext, ok := ctxValue.(common.AppContext); ok {
				// clean up temp directory if debug flag is not set
				if appContext.LocalTempDir != "" && !flagDebug {
					err := os.RemoveAll(appContext.LocalTempDir)
					if err != nil {
						slog.Error("error cleaning up temp directory", slog.String("tempDir", appContext.LocalTempDir), slog.String("error", err.Error()))
					}
				}
				slog.Info("Shutting down", slog.String("app", common.AppName), slog.String("version", gVersion), slog.Int("PID", os.Getpid()), slog.String("arguments", strings.Join(os.Args, " ")))
				if gLogFile != nil {
					err := gLogFile.Close()
					if err != nil {
						slog.Error("error closing log file", slog.String("logFile", gLogFile.Name()), slog.String("error", err.Error()))
						return err
					}
				}
			}
		}
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

const (
	updateCommandName = "update"
)

var updateCmd = &cobra.Command{
	GroupID: "other",
	Use:     updateCommandName,
	Short:   "Update the application",
	RunE: func(cmd *cobra.Command, args []string) error {
		appContext := cmd.Parent().Context().Value(common.AppContext{}).(common.AppContext)
		localTempDir := appContext.LocalTempDir
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
	// try both versioned and unversioned filenames, until we settle on a naming convention
	// confirm that manifest's version is a valid semver
	if !util.IsValidSemver(latestManifest.Version) {
		return fmt.Errorf("invalid version format in manifest: %s", latestManifest.Version)
	}
	versionedFileName := "perfspect" + "_" + latestManifest.Version + ".tgz"
	unVersionedfileName := "perfspect.tgz"
	fileNames := []string{unVersionedfileName, versionedFileName}
	var err error
	var resp *http.Response
	for _, fileName := range fileNames {
		url := artifactoryUrl + fileName
		resp, err = http.Get(url) // #nosec G107
		if err == nil && resp.StatusCode == http.StatusOK {
			slog.Info("Downloaded latest release", slog.String("url", url))
			break
		} else if err != nil {
			slog.Warn("Failed to download latest release", slog.String("url", url), slog.String("error", err.Error()))
		} else {
			slog.Warn("Failed to download latest release", slog.String("url", url), slog.String("status", resp.Status))
		}
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// write the tarball to a temp file
	tarballFile, err := os.CreateTemp(localTempDir, "perfspect*.tgz")
	if err != nil {
		return err
	}
	slog.Debug("Writing tarball to temp file", slog.String("tempFile", tarballFile.Name()))
	_, err = io.Copy(tarballFile, resp.Body)
	closeErr := tarballFile.Close()
	if err != nil {
		slog.Error("Error writing tarball to temp file", slog.String("tempFile", tarballFile.Name()), slog.String("error", err.Error()))
		return err
	}
	if closeErr != nil {
		slog.Error("Error closing tarball file", slog.String("tempFile", tarballFile.Name()), slog.String("error", closeErr.Error()))
		return closeErr
	}
	// rename the running app to "_<version>"
	oldAppFile := runningAppFile + "_" + gVersion
	oldAppPath := filepath.Join(runningAppDir, oldAppFile)
	slog.Info("Renaming running app", slog.String("from", runningAppFile), slog.String("to", oldAppFile))
	err = os.Rename(runningAppPath, oldAppPath)
	if err != nil {
		slog.Error("Error renaming running app", slog.String("from", runningAppFile), slog.String("to", oldAppFile), slog.String("error", err.Error()))
		return err
	}
	// rename the targets.yaml file to ".sav" if it exists
	targetsFile := filepath.Join(runningAppDir, "targets.yaml")
	if util.FileOrDirectoryExists(targetsFile) {
		slog.Info("Renaming targets file", slog.String("from", "targets.yaml"), slog.String("to", "targets.yaml.sav"))
		err = os.Rename(targetsFile, targetsFile+".sav")
		if err != nil {
			slog.Error("Error renaming targets file", slog.String("from", "targets.yaml"), slog.String("to", "targets.yaml.sav"), slog.String("error", err.Error()))
			return err
		}
	}
	// extract the tarball over the running app's directory
	slog.Info("Extracting latest release", slog.String("from", tarballFile.Name()), slog.String("to", runningAppDir))
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
		if util.FileOrDirectoryExists(targetsFile + ".sav") {
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
		slog.Error("Error removing tarball", slog.String("tempFile", tarballFile.Name()), slog.String("error", err.Error()))
		return err
	}
	// replace the new targets.yaml with the saved one
	if util.FileOrDirectoryExists(targetsFile + ".sav") {
		slog.Info("Restoring targets file", slog.String("from", "targets.yaml.sav"), slog.String("to", "targets.yaml"))
		err = os.Rename(targetsFile+".sav", targetsFile)
		if err != nil {
			slog.Error("Error restoring targets file", slog.String("from", "targets.yaml.sav"), slog.String("to", "targets.yaml"), slog.String("error", err.Error()))
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
	url := artifactoryUrl + "manifest.json"
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

// SyslogHandler is a slog.Handler that logs to syslog.
type SyslogHandler struct {
	writer     *syslog.Writer
	logLeveler slog.Leveler
	addSource  bool
}

func NewSyslogHandler(logOpts *slog.HandlerOptions) (*SyslogHandler, error) {
	writer, err := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, filepath.Base(os.Args[0]))
	if err != nil {
		return nil, err
	}
	return &SyslogHandler{writer: writer, logLeveler: logOpts.Level, addSource: logOpts.AddSource}, nil
}

func (h *SyslogHandler) Handle(ctx context.Context, r slog.Record) error {
	var msg string
	if r.PC != 0 && h.addSource {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		// get the file name with path relative to the current working directory + the last directory in the working directory
		filePath := f.File
		if strings.HasPrefix(filePath, "/") {
			wd, err := os.Getwd()
			if err == nil {
				filePath, err = filepath.Rel(wd, filePath)
				if err == nil {
					// last path element in working directory
					_, lastWd := filepath.Split(wd)
					filePath = filepath.Join(lastWd, filePath)
				} else {
					filePath = f.File
				}
			}
		}
		msg = fmt.Sprintf("level=%s source=%s:%d msg=\"%s\"", r.Level.String(), filePath, f.Line, r.Message)
	} else {
		msg = fmt.Sprintf("level=%s msg=\"%s\"", r.Level.String(), r.Message)
	}
	r.Attrs(func(attr slog.Attr) bool {
		msg += fmt.Sprintf(" %s=\"%s\"", attr.Key, attr.Value)
		return true
	})
	switch r.Level {
	case slog.LevelDebug:
		return h.writer.Debug(msg)
	case slog.LevelInfo:
		return h.writer.Info(msg)
	case slog.LevelWarn:
		return h.writer.Warning(msg)
	case slog.LevelError:
		return h.writer.Err(msg)
	default:
		return h.writer.Info(msg)
	}
}

func (h *SyslogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *SyslogHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *SyslogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.logLeveler.Level()
}

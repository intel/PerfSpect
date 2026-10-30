// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package telemetry

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"perfspect/internal/util"
)

// PDU telemetry is a hidden feature, configured entirely through environment variables.
// It is enabled when the host, user and password are set, along with either an outlet
// group or an explicit outlet list.
const (
	pduHostEnvVar       = "PERFSPECT_PDU_HOST"
	pduUserEnvVar       = "PERFSPECT_PDU_USER"
	pduPasswordEnvVar   = "PERFSPECT_PDU_PASSWORD" // #nosec G101 -- this is the name of an environment variable, not a credential
	pduOutletEnvVar     = "PERFSPECT_PDU_OUTLET"
	pduOutletListEnvVar = "PERFSPECT_PDU_OUTLET_LIST"
)

// maxPDUOutletNumber bounds the outlet numbers accepted in PERFSPECT_PDU_OUTLET_LIST.
// Raritan PDUs top out at a few dozen outlets; the bound keeps a typo such as "1-99999"
// from expanding into an absurd number of requests per sample.
const maxPDUOutletNumber = 256

// pduOutletNumberRe matches the individual numbers in an outlet list, so that each can
// be bounded before the list is expanded.
var pduOutletNumberRe = regexp.MustCompile(`\d+`)

// pduConfig holds the validated PDU telemetry configuration.
type pduConfig struct {
	host     string
	user     string
	password string
	// readPaths are PDU resource paths, relative to the model root and without a leading
	// slash, whose activePower readings are summed to produce one sample. Outlet group
	// mode yields exactly one path.
	readPaths []string
}

// enabled reports whether PDU telemetry should be collected.
func (c pduConfig) enabled() bool {
	return len(c.readPaths) > 0
}

// readings returns the number of readings that make up one sample.
func (c pduConfig) readings() int {
	return len(c.readPaths)
}

// bulkRequest returns the JSON body of a performBulk request reading activePower for
// every configured path, which the PDU answers in one round trip. It is built here, not
// in the script, so that it is valid JSON and contains nothing the script must quote.
func (c pduConfig) bulkRequest() (string, error) {
	type bulkSubRequest struct {
		RID  string         `json:"rid"`
		JSON map[string]any `json:"json"`
	}
	requests := make([]bulkSubRequest, 0, len(c.readPaths))
	for i, readPath := range c.readPaths {
		requests = append(requests, bulkSubRequest{
			RID: "/model/" + readPath + "/activePower",
			JSON: map[string]any{
				"jsonrpc": "2.0",
				"method":  "getReading",
				"id":      i + 1,
			},
		})
	}
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  "performBulk",
		"id":      1,
		"params":  map[string]any{"requests": requests},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to build the PDU bulk request: %w", err)
	}
	// the script wraps the body in single quotes, so a single quote would break out of it
	if strings.ContainsAny(string(encoded), "'\\") {
		return "", fmt.Errorf("the PDU bulk request contains unexpected characters: %s", encoded)
	}
	return string(encoded), nil
}

// pduConfigFromEnv builds the PDU telemetry configuration from the environment. It
// returns a disabled config when PDU telemetry was not requested, and an error when it
// was requested but configured incorrectly, rather than skipping it silently.
func pduConfigFromEnv() (pduConfig, error) {
	host := os.Getenv(pduHostEnvVar)
	user := os.Getenv(pduUserEnvVar)
	password := os.Getenv(pduPasswordEnvVar)
	outletGroup := os.Getenv(pduOutletEnvVar)
	outletList := os.Getenv(pduOutletListEnvVar)

	// nothing set, PDU telemetry not requested
	if host == "" && user == "" && password == "" && outletGroup == "" && outletList == "" {
		return pduConfig{}, nil
	}
	// treat a partially configured PDU as "not requested" to preserve the previous
	// behavior, where PDU telemetry was enabled only when every variable was set
	if host == "" || user == "" || password == "" || (outletGroup == "" && outletList == "") {
		slog.Debug("PDU telemetry not enabled, incomplete configuration",
			slog.Bool("host", host != ""),
			slog.Bool("user", user != ""),
			slog.Bool("password", password != ""),
			slog.Bool("outlet", outletGroup != ""),
			slog.Bool("outletList", outletList != ""))
		return pduConfig{}, nil
	}
	if err := validatePDUHost(host); err != nil {
		return pduConfig{}, err
	}

	config := pduConfig{host: host, user: user, password: password}
	// the outlet list takes precedence over the outlet group when both are set
	if outletList != "" {
		if outletGroup != "" {
			slog.Warn("both "+pduOutletEnvVar+" and "+pduOutletListEnvVar+" are set, using the outlet list and ignoring the outlet group",
				slog.String("outletListUsed", outletList),
				slog.String("outletGroupIgnored", outletGroup))
		}
		outlets, err := parsePDUOutletList(outletList)
		if err != nil {
			return pduConfig{}, err
		}
		for _, outlet := range outlets {
			config.readPaths = append(config.readPaths, pduOutletPath(outlet))
		}
		return config, nil
	}
	group, err := strconv.Atoi(outletGroup)
	if err != nil || group < 0 {
		return pduConfig{}, fmt.Errorf("%s must be a non-negative integer, got %q", pduOutletEnvVar, outletGroup)
	}
	// the path is built from the parsed number rather than the supplied text, so that
	// the value interpolated into the request cannot be anything but digits
	config.readPaths = []string{fmt.Sprintf("outletgroup/%d", group)}
	return config, nil
}

// logSummary logs the enabled PDU configuration. The password is deliberately omitted.
func (c pduConfig) logSummary() {
	slog.Info("PDU telemetry enabled",
		slog.String("host", c.host),
		slog.Int("readings", len(c.readPaths)),
		slog.String("paths", strings.Join(c.readPaths, " ")))
}

// pduOutletPath returns the PDU resource path for an individual outlet. Outlet numbers
// are 1-based as labeled on the PDU, while the JSON-RPC model is 0-based. It converts
// in one place only, so it is a one-line change if a PDU disagrees.
func pduOutletPath(outletNumber int) string {
	return fmt.Sprintf("outlet/%d", outletNumber-1)
}

// validatePDUHost rejects hosts that are not usable in the reading URL. A protocol
// prefix is rejected rather than stripped because the script also derives a no_proxy
// value from this string.
func validatePDUHost(host string) error {
	if strings.Contains(host, "://") {
		return fmt.Errorf("%s must not include a protocol prefix, got %q", pduHostEnvVar, host)
	}
	if strings.ContainsAny(host, " \t\n\"'$`\\;&|<>(){}[]*?!#") || strings.Contains(host, "/") {
		return fmt.Errorf("%s contains invalid characters, got %q", pduHostEnvVar, host)
	}
	return nil
}

// parsePDUOutletList expands a comma-separated list of outlet numbers and ranges into a
// sorted, de-duplicated list of outlet numbers. Accepted forms are "1", "1,2,5", "1-8"
// and combinations such as "1,4-6,9".
func parsePDUOutletList(value string) ([]int, error) {
	// the expansion is whitespace sensitive, so tolerate a spaced-out list here
	compact := strings.Join(strings.Fields(value), "")
	// Bound every number before expanding the list, because expansion allocates one
	// entry per outlet in a range and so would spend a great deal of time and memory
	// on a mistyped bound such as "1-2000000000".
	for _, number := range pduOutletNumberRe.FindAllString(compact, -1) {
		outlet, err := strconv.Atoi(number)
		// outlets are labeled from 1 on the PDU
		if err != nil || outlet < 1 || outlet > maxPDUOutletNumber {
			return nil, fmt.Errorf("%s outlet number %q is out of range, expected 1-%d", pduOutletListEnvVar, number, maxPDUOutletNumber)
		}
	}
	outlets, err := util.SelectiveIntRangeToIntList(compact)
	if err != nil {
		return nil, fmt.Errorf("%s is not a list of outlet numbers or ranges, got %q: %w", pduOutletListEnvVar, value, err)
	}
	if len(outlets) == 0 {
		return nil, fmt.Errorf("%s does not specify any outlets, got %q", pduOutletListEnvVar, value)
	}
	slices.Sort(outlets)
	return slices.Compact(outlets), nil
}

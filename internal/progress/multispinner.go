// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

/*
Package progress provides CLI progress bar options.
*/
package progress

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

var spinChars []string = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

type MultiSpinnerUpdateFunc func(string, string) error

type spinnerState struct {
	label       string
	status      string
	statusIsNew bool
	spinIndex   int
}

type multiSpinner struct {
	spinners []spinnerState
	ticker   *time.Ticker
	done     chan bool
	spinning bool
}

// NewMultiSpinner creates a new MultiSpinner
func NewMultiSpinner() *multiSpinner {
	ms := multiSpinner{}
	ms.done = make(chan bool)
	return &ms
}

// AddSpinner adds a spinner to the MultiSpinner
func (ms *multiSpinner) AddSpinner(label string) (err error) {
	// make sure label is unique
	for _, spinner := range ms.spinners {
		if spinner.label == label {
			err = fmt.Errorf("spinner with label %s already exists", label)
			return
		}
	}
	ms.spinners = append(ms.spinners, spinnerState{label, "?", false, 0})
	return
}

// Start starts the spinner
func (ms *multiSpinner) Start() {
	ms.draw(true)
	ms.ticker = time.NewTicker(250 * time.Millisecond)
	ms.spinning = true
	go ms.onTick()
}

// Finish stops the spinner
func (ms *multiSpinner) Finish() {
	if ms.spinning {
		ms.ticker.Stop()
		ms.done <- true
		ms.draw(false)
		ms.spinning = false
	}
}

// Status updates the status of a spinner
func (ms *multiSpinner) Status(label string, status string) (err error) {
	for spinnerIdx, spinner := range ms.spinners {
		if spinner.label == label {
			if status != spinner.status {
				ms.spinners[spinnerIdx].status = status
				ms.spinners[spinnerIdx].statusIsNew = true
			}
			return
		}
	}
	err = fmt.Errorf("did not find spinner with label %s", label)
	return
}

func (ms *multiSpinner) onTick() {
	for {
		select {
		case <-ms.done:
			return
		case <-ms.ticker.C:
			ms.draw(true)
		}
	}
}

func (ms *multiSpinner) draw(goUp bool) {
	for i, spinner := range ms.spinners {
		if !term.IsTerminal(int(os.Stderr.Fd())) && !spinner.statusIsNew {
			continue
		}
		fmt.Fprintf(os.Stderr, "%-20s  %s  %-40s\n", spinner.label, spinChars[spinner.spinIndex], spinner.status)
		ms.spinners[i].statusIsNew = false
		ms.spinners[i].spinIndex += 1
		if ms.spinners[i].spinIndex >= len(spinChars) {
			ms.spinners[i].spinIndex = 0
		}
	}
	if goUp && term.IsTerminal(int(os.Stderr.Fd())) {
		for range ms.spinners {
			fmt.Fprintf(os.Stderr, "\x1b[1A")
		}
	}
}

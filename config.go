// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import (
	"os"
	"encoding/json"
	"github.com/pterm/pterm"
)

var (
	debugMode    = false
	autoYes      = false
	targetBranch = "stable"
)

const (
	StoreBase    = "/opt/una"
	BinPath      = "/usr/local/bin"
	Applications = "/usr/share/applications"
	DefaultIcon  = "/usr/share/defaults/icon.png"
	LockFile     = "/tmp/una.lock"
	ConfigDir    = "/etc/una"
	SourcesFile  = "/etc/una/sources.json"
)

var unaBarStyle = pterm.DefaultProgressbar.
	WithBarStyle(pterm.NewStyle(pterm.FgCyan)).
	WithTitleStyle(pterm.NewStyle(pterm.FgLightCyan)).
	WithBarCharacter("━").
	WithLastCharacter(" ").
	WithBarFiller("─").
	WithShowCount(false).
	WithShowPercentage(true).
	WithRemoveWhenDone(false)

func loadSources() []RepoSource {
	defaultRepo := RepoSource{URL: "https://raw.githubusercontent.com/UincOS/Packages", Branch: "stable"}
	if _, err := os.Stat(SourcesFile); os.IsNotExist(err) {
		return []RepoSource{defaultRepo}
	}
	data, _ := os.ReadFile(SourcesFile)
	var sources []RepoSource
	_ = json.Unmarshal(data, &sources)
	return sources
}
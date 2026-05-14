// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 [Your Name/Entity]. All rights reserved.

package main

import "github.com/pterm/pterm"

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

type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Exec         string   `json:"exec"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Website      string   `json:"website"`
	License      string   `json:"license"`
	Dependencies []string `json:"dependencies"`
}

type RepoSource struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

type RepoSchema struct {
	Stable struct {
		Main    []RepoPackage `json:"main"`
		Contrib []RepoPackage `json:"contrib"`
		NonFree []RepoPackage `json:"non-free"`
	} `json:"stable"`
}

type RepoPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
	Hash    string `json:"hash"`
}

type InstallStep struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Command string `json:"command,omitempty"`
}

type InstallManifest struct {
	Steps []InstallStep `json:"steps"`
}
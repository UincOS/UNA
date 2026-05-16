// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import (
	"os"
	"fmt"
	"strings"
	"encoding/json"
	"path/filepath"
	"github.com/pterm/pterm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func showHelp() {
	table := pterm.TableData{
		[]string{"Command", "Description"},
		[]string{"-i, install", "Install a package"},
		[]string{"-u, update", "Update a package"},
		[]string{"-r, remove", "Remove a package"},
		[]string{"-l, list", "List installed packages"},
		[]string{"-info, information", "Show package details"},
	}
	_ = pterm.DefaultTable.WithData(table).Render()
}

func showAppInfo(name string) {
	if getVersionInstalled(name) == "" {
		pterm.Error.Printf("- Package '%s' is not installed.\n", name)
		return
	}

	raw, err := os.ReadFile(filepath.Join(StoreBase, name, "metadata.json"))
	if err != nil {
		pterm.Error.Println("- Metadata file is missing or corrupted.")
		return
	}
	var m Metadata
	_ = json.Unmarshal(raw, &m)
	pterm.DefaultSection.Println("Package Details: " + m.Name)
	fmt.Printf("Version:     %s\nAuthor:      %s\nWebsite:     %s\nDescription: %s\n", m.Version, m.Author, m.Website, m.Description)
}

func listPackages() {
	if _, err := os.Stat(StoreBase); os.IsNotExist(err) {
		pterm.Warning.Println("/ No packages installed.")
		return
	}

	files, _ := os.ReadDir(StoreBase)
	if len(files) == 0 {
		pterm.Warning.Println("/ No packages installed.")
		return
	}

	tableData := pterm.TableData{[]string{"Package", "Version", "Author"}}
	found := false
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(StoreBase, f.Name(), "metadata.json"))
		if err == nil {
			var m Metadata
			_ = json.Unmarshal(raw, &m)
			tableData = append(tableData, []string{m.Name, m.Version, m.Author})
			found = true
		}
	}

	if !found {
		pterm.Warning.Println("/ No valid packages found.")
		return
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
}

func handleAddRepo(args []string) {
	if os.Geteuid() != 0 {
		pterm.Error.Println("- Root required.")
		return
	}
	if len(args) < 2 {
		return
	}
	url := strings.TrimSuffix(args[1], "/")
	sources := loadSources()
	sources = append(sources, RepoSource{URL: url, Branch: targetBranch})
	data, _ := json.MarshalIndent(sources, "", "  ")
	_ = os.WriteFile(SourcesFile, data, 0644)
	pterm.Success.Printf("+ Repository added: %s\n", url)
}

func handleInstall(args []string, isUpdate bool, force bool) {
	if len(args) < 2 {
		pterm.Error.Println("- Package name or .una file required.")
		return
	}
	target := args[1]

	if (isUpdate || force) && !strings.HasSuffix(target, ".una") {
		if getVersionInstalled(target) == "" {
			pterm.Error.Printf("- Package '%s' is not installed. Use 'una -i %s' to install.", target, target)
			return
		}
	}

	if !isUpdate && !force {
		var appName string
		if strings.HasSuffix(target, ".una") {
			appName, _ = getLocalFileInfo(target)
		} else {
			appName = target
		}

		currentVersion := getVersionInstalled(appName)
		if currentVersion != "" {
			pterm.Warning.Printf("/ Package '%s' is already installed (Version: %s).\n", appName, currentVersion)
			pterm.Info.Println("+ Use 'una -u <app>' to update or 'una -f <app>' to fix.")
			return
		}
	}

	if !askConfirmation(fmt.Sprintf("Confirm %s of '%s'?", getActionState(isUpdate, force), target)) {
		return
	}
	pterm.Println()

	if strings.HasSuffix(target, ".una") {
		installPackage(target, isUpdate, force, nil)
	} else {
		fetchAndInstall(target, isUpdate, force)
	}
}

func handleRemove(args []string) {
	if os.Geteuid() != 0 {
		pterm.Println()
		pterm.Error.Println("Root privileges required to remove packages.")
		return
	}
	if len(args) < 2 {
		pterm.Println()
		pterm.Error.Println("Usage: una -r <package>")
		return
	}
	name := args[1]

	if getVersionInstalled(name) == "" {
		pterm.Println()
		pterm.Error.Printf("- Package '%s' not found. Nothing to remove.\n", name)
		return
	}

	if !askConfirmation(fmt.Sprintf("Remove '%s'?", name)) {
		pterm.Println()
		return
	}
	pterm.Println()

	appDir := filepath.Join(StoreBase, name)
	binLink := filepath.Join(BinPath, name)
	desktopFile := filepath.Join(Applications, "una-"+name+".desktop")

	totalItems := 0
	_ = filepath.Walk(appDir, func(_ string, info os.FileInfo, err error) error {
		if err == nil {
			totalItems++
		}
		return nil
	})
	if _, err := os.Stat(binLink); err == nil {
		totalItems++
	}
	if _, err := os.Stat(desktopFile); err == nil {
		totalItems++
	}

	totalItems++

	p, _ := unaBarStyle.
		WithTotal(totalItems).
		WithTitle(fmt.Sprintf("Uninstalling %s", name)).
		Start()

	var filesToDelete []string
	_ = filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			filesToDelete = append(filesToDelete, path)
		}
		return nil
	})

	for i := len(filesToDelete) - 1; i >= 0; i-- {
		path := filesToDelete[i]
		p.UpdateTitle(fmt.Sprintf("Deleting: %s", filepath.Base(path)))
		_ = os.Remove(path)
		p.Increment()
	}
	pterm.Info.Printfln("- Removed files for: %s", name)

	if _, err := os.Stat(binLink); err == nil {
		p.UpdateTitle("Removing binary link...")
		_ = os.Remove(binLink)
		p.Increment()
		pterm.Info.Printf("- Removed binary link: %s\n", name)
	}

	if _, err := os.Stat(desktopFile); err == nil {
		p.UpdateTitle("Removing desktop shortcut...")
		_ = os.Remove(desktopFile)
		p.Increment()
		pterm.Info.Printf("- Removed desktop entry: %s\n", name)
	}

	actionState := "Removed"
	caser := cases.Title(language.English)
	p.UpdateTitle(fmt.Sprintf("%-12s", caser.String(actionState)))
	pterm.Success.Printf("- Package was successfully removed: %s\n", name)
	pterm.Println()
	p.Increment()
	p.Stop()
}

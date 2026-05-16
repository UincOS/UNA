// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import (
	"os"
	"fmt"
	"strings"
	"syscall"
	"os/exec"
	"encoding/json"
	"path/filepath"
	"github.com/pterm/pterm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func launchApp(appName string) {
	appDir := filepath.Join(StoreBase, appName)
	metaPath := filepath.Join(appDir, "metadata.json")

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		pterm.Error.Printf("- Application %s not found in store (%s).", appName, StoreBase)
		os.Exit(1)
	}

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		pterm.Error.Printf("- Failed to read metadata for %s. Package might be corrupted.", appName)
		os.Exit(1)
	}
	var meta Metadata
	_ = json.Unmarshal(raw, &meta)

	binDir := filepath.Join(appDir, "bin")
	binPath := filepath.Join(binDir, meta.Exec)
	libDir := filepath.Join(appDir, "lib")

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		pterm.Error.Printf("- Executable binary '%s' not found at %s", meta.Exec, binPath)
		os.Exit(1)
	}

	rawEnv := os.Environ()
	var finalEnv []string

	ldPaths := []string{binDir, libDir}
	if sysLd := os.Getenv("LD_LIBRARY_PATH"); sysLd != "" {
		ldPaths = append(ldPaths, sysLd)
	}
	newLdLibraryPath := "LD_LIBRARY_PATH=" + strings.Join(ldPaths, ":")

	for _, envVar := range rawEnv {
		if !strings.HasPrefix(envVar, "LD_LIBRARY_PATH=") {
			finalEnv = append(finalEnv, envVar)
		}
	}
	finalEnv = append(finalEnv, newLdLibraryPath)

	appArgs := []string{binPath}
	if len(os.Args) > 3 {
		appArgs = append(appArgs, os.Args[3:]...)
	}

	pterm.Debug.Printf("> Launching %s with LD_LIBRARY_PATH isolation...", appName)

	_ = os.Chdir(binDir)

	err = syscall.Exec(binPath, appArgs, finalEnv)

	if err != nil {
		pterm.Error.Printf("- Critical failure launching %s: %v", appName, err)
		os.Exit(1)
	}
}

func createDesktopShortcut(meta Metadata, appPath string) {
    matches, _ := filepath.Glob(filepath.Join(appPath, meta.Name+".*"))

    icon := ""
    if len(matches) > 0 {
        icon = matches[0]
    } else {
        icon = filepath.Join(appPath, "icon.png")
    }

    caser := cases.Title(language.English)
    displayName := caser.String(meta.Name)

    category := meta.Category
    if category == "" {
        category = "Utility"
    }
    if !strings.HasSuffix(category, ";") {
        category += ";"
    }

    content := fmt.Sprintf("[Desktop Entry]\nName=%s\nExec=%s/%s\nIcon=%s\nType=Application\nCategories=%s\nComment=%s\nTerminal=false",
        displayName, BinPath, meta.Name, icon, category, meta.Description)

    _ = os.MkdirAll(Applications, 0755)
    _ = os.WriteFile(filepath.Join(Applications, "una-"+meta.Name+".desktop"), []byte(content), 0644)
}

func runCustomInstall(jsonPath string, appPath string) error {
    data, err := os.ReadFile(jsonPath)
    if err != nil {
        pterm.Error.Printf("- Failed to read installation manifest: %v\n", err)
        return err
    }

    var manifest InstallManifest
    _ = json.Unmarshal(data, &manifest)

    binDir := filepath.Join(appPath, "bin")
    
    userHome := os.Getenv("HOME")
    if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
        userHome = filepath.Join("/home", sudoUser)
    }

    for _, step := range manifest.Steps {
        pterm.Info.Printf("+ Step: %s\n", step.Name)

        switch step.Type {
        case "download":
            err := downloadWithProgress(step.URL, step.Dest, "Downloading external resource")
            if err != nil {
                pterm.Error.Printf("- Download failed: %v\n", err)
                return err
            }

        case "command":
            cmdStr := step.Command
            
            cmdStr = strings.ReplaceAll(cmdStr, "<BIN_DIR>", binDir)
            cmdStr = strings.ReplaceAll(cmdStr, "<APPS_DIR>", appPath)
            cmdStr = strings.ReplaceAll(cmdStr, "<USER_HOME>", userHome)
            
            cmd := exec.Command("bash", "-c", cmdStr)
            
            err := cmd.Run()
            if err != nil {
                pterm.Error.Printf("- Execution failed for step: %s\n", step.Name)
                return err
            }
        }
    }
    
    return nil
}

func findSystemLib(libName string) string {
	paths := []string{
		"/usr/lib/x86_64-linux-gnu",
		"/lib/x86_64-linux-gnu",
		"/usr/lib",
		"/lib",
		"/usr/lib64",
		"/lib64",
	}
	for _, p := range paths {
		fullPath := filepath.Join(p, libName)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return ""
}
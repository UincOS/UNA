// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 [Your Name/Entity]. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"github.com/pterm/pterm"
)

func main() {
	binaryName := filepath.Base(os.Args[0])
	if binaryName != "una" {
		launchApp(binaryName)
		return
	}

	args := os.Args[1:]
	cleanArgs := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-d": debugMode = true
		case "-y": autoYes = true
		case "-b":
			if i+1 < len(args) {
				targetBranch = args[i+1]
				i++
			}
		default: cleanArgs = append(cleanArgs, arg)
		}
	}

	pterm.Println(pterm.LightCyan("Una Package Manager"))

	if len(cleanArgs) < 1 {
		showHelp()
		return
	}

	command := cleanArgs[0]
	writeCommands := map[string]bool{"-i": true, "install": true, "-u": true, "update": true, "-r": true, "remove": true, "-f": true, "fix": true, "add-repo": true}
	
	if writeCommands[command] {
		if err := acquireLock(); err != nil {
			pterm.Error.Println(err)
			return
		}
		defer releaseLock()
	}

	switch command {
	case "-i", "install": handleInstall(cleanArgs, false, false)
	case "-u", "update":  handleInstall(cleanArgs, true, false)
	case "-f", "fix":     handleInstall(cleanArgs, true, true)
	case "-r", "remove":  handleRemove(cleanArgs)
	case "add-repo":      handleAddRepo(cleanArgs)
	case "-l", "list":    listPackages()
	case "-info":         showAppInfo(cleanArgs[1])
	default:              showHelp()
	}
}
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"golang.org/x/text/cases"
    "golang.org/x/text/language"
	"github.com/pterm/pterm"
)

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
        if err == nil { totalItems++ }
        return nil
    })
    if _, err := os.Stat(binLink); err == nil { totalItems++ }
    if _, err := os.Stat(desktopFile); err == nil { totalItems++ }
    
    totalItems++ 

    p, _ := unaBarStyle.
        WithTotal(totalItems).
        WithTitle(fmt.Sprintf("Uninstalling %s", name)).
        Start()
    
    var filesToDelete []string
    _ = filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
        if err == nil { filesToDelete = append(filesToDelete, path) }
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
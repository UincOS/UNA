// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import (
	"os"
	"io"
	"fmt"
	"strings"
	"syscall"
	"os/exec"
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"path/filepath"
	"github.com/pterm/pterm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func installPackage(unaFile string, isUpdate bool, force bool, globalBar *pterm.ProgressbarPrinter) {
	absPath, _ := filepath.Abs(unaFile)

	info, err := os.Stat(absPath)
	if err == nil && info.Size() < 500 {
		if globalBar != nil {
			globalBar.Stop()
		}
		pterm.Error.Println("- The file is too small (< 500 bytes). It might be a Git LFS pointer or a corrupted package.")
		return
	}

	var stat syscall.Statfs_t
	syscall.Statfs("/", &stat)
	storageBytes := stat.Bavail * uint64(stat.Bsize)
	unaFileSize, err := os.Stat(absPath)
	packageBytes := unaFileSize.Size()

	if verifyStorage(packageBytes, storageBytes) {
		if globalBar != nil {
			globalBar.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
			pterm.Error.Printfln("- Not enough storage for %s. %v < %v", getActionVerb(isUpdate, force), toMegaByte(uint64(storageBytes)), toMegaByte(uint64(packageBytes)))
			globalBar.Increment()
			globalBar.Stop()
			return
		}
	} else {
		if debugMode == true {
			pterm.Debug.Println()
		}
	}

	f, err := os.Open(absPath)
	if err != nil {
		if globalBar != nil {
			globalBar.Stop()
		}
		pterm.Error.Printf("- Failed to open package: %v\n", err)
		return
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		if globalBar != nil {
			globalBar.Stop()
		}
		pterm.Error.Printf("- Invalid gzip format: %v\n", err)

		sniff := make([]byte, 512)
		_, _ = f.ReadAt(sniff, 0)
		if strings.Contains(string(sniff), "<!DOCTYPE html") || strings.Contains(string(sniff), "<html>") {
			pterm.Warning.Println("/ The file is an HTML page, not a package. Please check repository URL in sources.")
		}
		return
	}
	tr := tar.NewReader(gz)

	var meta Metadata
	totalFiles := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		totalFiles++
		if filepath.Base(header.Name) == "metadata.json" {
			content, _ := io.ReadAll(tr)
			_ = json.Unmarshal(content, &meta)
		}
	}
	gz.Close()

	isLocal := false
	if globalBar == nil {
		isLocal = true
		globalBar, _ = unaBarStyle.
			WithTotal(totalFiles + 3).
			WithTitle(fmt.Sprintf("%-12s", "Installing")).
			Start()

		pterm.Info.Printf("+ Founded local metadata for: %s\n", meta.Name)
		globalBar.Increment()
	} else {
		globalBar.Total = totalFiles + 3
	}

	_, _ = f.Seek(0, 0)
	gz, err = gzip.NewReader(f)
	if err != nil {
		if globalBar != nil {
			globalBar.Stop()
		}
		pterm.Error.Printf("- Failed to reset gzip reader: %v\n", err)
		return
	}
	defer gz.Close()
	tr = tar.NewReader(gz)

	appPath := filepath.Join(StoreBase, meta.Name)
	if isUpdate || force {
		_ = os.RemoveAll(appPath)
	}
	_ = os.MkdirAll(appPath, 0755)

	ioBuffer := make([]byte, 1024*1024)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		target := filepath.Join(appPath, header.Name)

		if globalBar != nil {
			globalBar.UpdateTitle(fmt.Sprintf("Extracting: %s", filepath.Base(header.Name)))
		}

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0755)

		case tar.TypeSymlink:
			_ = os.Remove(target)
			err := os.Symlink(header.Linkname, target)
			if err != nil {
				pterm.Warning.Printf("/ Error creating link %s: %v\n", target, err)
			}

		case tar.TypeReg:
			if force && isFileOk(target, header.Size) {
				if globalBar != nil {
					globalBar.Increment()
				}
				_, _ = io.Copy(io.Discard, tr)
				continue
			}
			_ = os.MkdirAll(filepath.Dir(target), 0755)

			out, err := os.Create(target)
			if err == nil {
				_, _ = io.CopyBuffer(out, tr, ioBuffer)
				out.Close()

				if strings.Contains(header.Name, "bin/") ||
					strings.Contains(header.Name, "lib/") ||
					strings.HasSuffix(header.Name, ".sh") ||
					strings.HasSuffix(header.Name, ".run") {
					_ = os.Chmod(target, 0755)
				}
			}
		default:
			pterm.Debug.Printf("> Unsupported file type: %v in %s\n", header.Typeflag, header.Name)
		}

		if globalBar != nil {
			globalBar.Increment()
		}
	}

	if isLocal {
		pterm.Info.Printfln("+ Extracted files for: %s", meta.Name)
		globalBar.Increment()
	}

	scriptPath := filepath.Join(appPath, "bin", "install.json")
    if _, err := os.Stat(scriptPath); err == nil {
        if err := runCustomInstall(scriptPath, appPath); err != nil {
            _ = os.RemoveAll(appPath)
            globalBar.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
            pterm.Warning.Println("/ Custom installation failed. Rolling back...")
            globalBar.Increment()
            globalBar.Stop()
            return
        }
    }

	unaBin, _ := exec.LookPath("una")
	linkPath := filepath.Join(BinPath, meta.Name)
	_ = os.Remove(linkPath)
	_ = os.Symlink(unaBin, linkPath)

	createDesktopShortcut(meta, appPath)

	actionState := getActionState(isUpdate, force)
	caser := cases.Title(language.English)

	globalBar.UpdateTitle(fmt.Sprintf("%-12s", caser.String(actionState)))
	pterm.Info.Printfln("+ Created desktop entry for: %s", meta.Name)
	pterm.Success.Printf("+ Package was successfully %s: %s\n", meta.Name, actionState)
	pterm.Println()
	globalBar.Increment()
	globalBar.Stop()
}
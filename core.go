// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import (
	"os"
	"io"
	"fmt"
	"strings"
	"os/exec"
	"net/http"
	"archive/tar"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"crypto/sha256"
	"github.com/pterm/pterm"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func fetchAndInstall(appName string, isUpdate bool, force bool) {
	if os.Geteuid() != 0 {
		pterm.Error.Println("- Root privileges required.")
		return
	}

	actionVerb := getActionVerb(isUpdate, force)
	globalProgress, _ := unaBarStyle.WithTotal(3).WithTitle(fmt.Sprintf("%-12s", actionVerb)).Start()

	targetPkg, selectedSource, err := FetchAllRemotePackages(appName)
    if err != nil {
        globalProgress.Stop()
        pterm.Error.Printf("- %v\n", err)
        return
    }

	pterm.Info.Printf("+ Retrieved Metadata for: %s\n", targetPkg.Name)
	globalProgress.Increment()

	baseURL := strings.TrimSuffix(selectedSource.URL, "/")
    fileURL := fmt.Sprintf("%s/%s/%s", baseURL, selectedSource.Branch, targetPkg.URL)

	fileResp, err := http.Get(fileURL)
	if err != nil || fileResp.StatusCode != 200 {
        globalProgress.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
		pterm.Error.Printf("- Download failed. URL: %s\n", fileURL)
        globalProgress.Increment()
        globalProgress.Stop()
		return
	}
	defer fileResp.Body.Close()

	hasher := sha256.New()
	teeReader := io.TeeReader(fileResp.Body, hasher)

	pterm.Info.Printfln("+ Established download stream for: %s", targetPkg.Name)
	globalProgress.Increment()
	globalProgress.UpdateTitle(fmt.Sprintf("%-12s", "Extracting"))

	gz, err := gzip.NewReader(teeReader)
	if err != nil {
        globalProgress.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
		pterm.Error.Println("- Invalid package format from server.")
        globalProgress.Increment()
        globalProgress.Stop()
		return
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	appPath := filepath.Join(StoreBase, targetPkg.Name)

	if isUpdate && !force {
		_ = os.RemoveAll(appPath)
	}
	_ = os.MkdirAll(appPath, 0755)

	var meta Metadata
	meta.Name = targetPkg.Name
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
		globalProgress.UpdateTitle(fmt.Sprintf("Extracting: %s", filepath.Base(header.Name)))

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0755)
		case tar.TypeSymlink:
			_ = os.Remove(target)
			_ = os.Symlink(header.Linkname, target)
		case tar.TypeReg:
			if force && isFileOk(target, header.Size) {
				_, _ = io.Copy(io.Discard, tr)
				continue
			}

			_ = os.MkdirAll(filepath.Dir(target), 0755)

			if filepath.Base(header.Name) == "metadata.json" {
				content, _ := io.ReadAll(tr)
				_ = json.Unmarshal(content, &meta)
				_ = os.WriteFile(target, content, 0644)
				continue
			}

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
		}
	}

	downloadedHash := hex.EncodeToString(hasher.Sum(nil))
    if !strings.EqualFold(downloadedHash, targetPkg.Hash) {
        pterm.Error.Printfln("\n- Security Error! Package hash mismatch.\n")
        pterm.Error.Printfln("- Expected : %s\n", targetPkg.Hash)
        pterm.Error.Printfln("- Received : %s\n", downloadedHash)
        
        _ = os.RemoveAll(appPath)
        globalProgress.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
        pterm.Error.Println("- Installation aborted and files cleaned up.")
        globalProgress.Increment()
        globalProgress.Stop()
        return
    }
    setAppHash(appPath, downloadedHash)

	scriptPath := filepath.Join(appPath, "bin", "install.json")
    if _, err := os.Stat(scriptPath); err == nil {
        if err := runCustomInstall(scriptPath, appPath); err != nil {
            _ = os.RemoveAll(appPath)
            globalProgress.UpdateTitle(fmt.Sprintf("%-12s", "Aborted"))
            pterm.Warning.Println("/ Custom installation failed. Rolling back...")
            globalProgress.Increment()
            globalProgress.Stop()
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

	globalProgress.UpdateTitle(fmt.Sprintf("%-12s", caser.String(actionState)))
	pterm.Info.Printfln("+ Created desktop entry for: %s", meta.Name)
	pterm.Success.Printf("+ Package was successfully %s: %s\n", actionState, meta.Name)
	pterm.Println()
	globalProgress.Increment()
	globalProgress.Stop()
}

func getLocalFileInfo(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	gz, _ := gzip.NewReader(f)
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if filepath.Base(header.Name) == "metadata.json" {
			var m Metadata
			content, _ := io.ReadAll(tr)
			_ = json.Unmarshal(content, &m)
			return m.Name, m.Version
		}
	}
	return "", ""
}

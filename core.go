/*
 * Copyright (c) 2026 Altha36. All Rights Reserved.
 * Licensed under the Altha Project License (APL) v1.2.
 * This component is part of the Proprietary Core of Altha36.
 * Unauthorized copying or distribution is strictly prohibited.
 */

package main

import (
    "archive/tar"
    "compress/gzip"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "syscall"
    "crypto/sha256"
    "encoding/hex"
    "golang.org/x/text/cases"
    "golang.org/x/text/language"
    "github.com/pterm/pterm"
)
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

func fetchAndInstall(appName string, isUpdate bool, force bool) {
    if os.Geteuid() != 0 {
        pterm.Error.Println("Root privileges required.")
        return
    }

    sources := loadSources()
    
    actionVerb := getActionVerb(isUpdate, force)
    globalProgress, _ := unaBarStyle.WithTotal(3).WithTitle(fmt.Sprintf("%-12s", actionVerb)).Start()

    var targetPkg *RepoPackage
    var selectedSource RepoSource

    for _, source := range sources {
        branch := source.Branch
        if targetBranch != "stable" { branch = targetBranch }
        repoURL := fmt.Sprintf("%s/%s/packages.json", source.URL, branch)
        resp, err := http.Get(repoURL)
        if err != nil || resp.StatusCode != 200 { continue }
        var repo RepoSchema
        err = json.NewDecoder(resp.Body).Decode(&repo)
        resp.Body.Close()
        if err != nil { continue }

        all := append(repo.Stable.Main, append(repo.Stable.Contrib, repo.Stable.NonFree...)...)
        for _, p := range all {
            if strings.EqualFold(p.Name, appName) {
                targetPkg = &p
                selectedSource = source
                selectedSource.Branch = branch
                break
            }
        }
        if targetPkg != nil { break }
    }

    if targetPkg == nil {
        globalProgress.Stop()
        pterm.Error.Println("- Package not found in repositories.")
        return
    }

    pterm.Info.Printf("+ Retrieved Metadata for: %s\n", targetPkg.Name)
    globalProgress.Increment()

    baseURL := strings.TrimSuffix(selectedSource.URL, "/")
    fileURL := fmt.Sprintf("%s/%s/%s", baseURL, selectedSource.Branch, targetPkg.URL)

    fileResp, err := http.Get(fileURL)
    if err != nil || fileResp.StatusCode != 200 {
        globalProgress.Stop()
        pterm.Error.Printf("- Download failed. URL: %s\n", fileURL)
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
        globalProgress.Stop()
        pterm.Error.Println("- Invalid package format from server.")
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
        if err == io.EOF { break }
        if err != nil { break }

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
                strings.Contains(header.Name, "libs/") || 
                strings.HasSuffix(header.Name, ".sh") || 
                strings.HasSuffix(header.Name, ".run") {
                    _ = os.Chmod(target, 0755)
                }
            }
        }
    }

    downloadedHash := hex.EncodeToString(hasher.Sum(nil))
    if !strings.EqualFold(downloadedHash, targetPkg.Hash) {
        pterm.Warning.Printfln("\n/ Hash mismatch! Expected: %s, Got: %s", targetPkg.Hash[:8], downloadedHash[:8])
    }
    setAppHash(appPath, downloadedHash)

    scriptPath := filepath.Join(appPath, "bin", "install.json")
    if _, err := os.Stat(scriptPath); err == nil { runCustomInstall(scriptPath, filepath.Join(appPath, "bin")) }
    
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

func installPackage(unaFile string, isUpdate bool, force bool, globalBar *pterm.ProgressbarPrinter) {
    absPath, _ := filepath.Abs(unaFile)

    info, err := os.Stat(absPath)
    if err == nil && info.Size() < 500 {
        if globalBar != nil { globalBar.Stop() }
        pterm.Error.Println("- The file is too small (< 500 bytes). It might be a Git LFS pointer or a corrupted package.")
        return
    }

    f, err := os.Open(absPath)
    if err != nil {
        if globalBar != nil { globalBar.Stop() }
        pterm.Error.Printf("- Failed to open package: %v\n", err)
        return
    }
    defer f.Close()

    gz, err := gzip.NewReader(f)
    if err != nil {
        if globalBar != nil { globalBar.Stop() }
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
        if err == io.EOF { break }
        if err != nil { break }
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
        if globalBar != nil { globalBar.Stop() }
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
        if err == io.EOF { break }
        if err != nil { break }

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
                if globalBar != nil { globalBar.Increment() }
                _, _ = io.Copy(io.Discard, tr)
                continue
            }
            _ = os.MkdirAll(filepath.Dir(target), 0755)
            
            if strings.Contains(header.Name, "libs/") {
                libName := filepath.Base(header.Name)
                sysLibPath := findSystemLib(libName)
                if sysLibPath != "" {
                    _ = os.Symlink(sysLibPath, target)
                    if globalBar != nil { globalBar.Increment() }
                    continue
                }
            }

            out, err := os.Create(target)
            if err == nil {
                _, _ = io.CopyBuffer(out, tr, ioBuffer)
                out.Close()
                
                if strings.Contains(header.Name, "bin/") || 
                   strings.Contains(header.Name, "libs/") || 
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
        runCustomInstall(scriptPath, filepath.Join(appPath, "bin"))
    }

    unaBin, _ := exec.LookPath("una")
    linkPath := filepath.Join(BinPath, meta.Name)
    _ = os.Remove(linkPath)
    _ = os.Symlink(unaBin, linkPath)

    createDesktopShortcut(meta, appPath)

    actionState := getActionState(isUpdate, force)
    caser := cases.Title(language.English)

    globalBar.UpdateTitle(fmt.Sprintf("%-12s", caser.String(actionState)))
    pterm.Info.Printfln("+ Created desktop entry for: %s\n", meta.Name)
    pterm.Success.Printf("+ Package was successfully %s: %s\n", meta.Name, actionState)
    pterm.Println()
    globalBar.Increment()
    globalBar.Stop()
}

func runCustomInstall(jsonPath string, binDir string) {
    data, _ := os.ReadFile(jsonPath)
    var manifest InstallManifest
    _ = json.Unmarshal(data, &manifest)

    for _, step := range manifest.Steps {
        if step.Type == "command" {
            cmdStr := strings.ReplaceAll(step.Command, "<BIN_DIR>", binDir)
            _ = exec.Command("bash", "-c", cmdStr).Run()
        }
    }
}

func downloadWithProgress(url, dest, title string) error {
    resp, err := http.Get(url)
    if err != nil { return err }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
    }

    out, err := os.Create(dest)
    if err != nil { return err }
    defer out.Close()

    totalSize := int(resp.ContentLength)
    if totalSize <= 0 {
        spinner, _ := pterm.DefaultSpinner.Start(title + " (Calculating size...)")
        _, err = io.Copy(out, resp.Body)
        if err != nil {
            spinner.Fail("Download error.")
            return err
        }
        spinner.Success(title)
        return nil
    }

    pb, _ := pterm.DefaultProgressbar.
        WithBarStyle(pterm.NewStyle(pterm.FgCyan)).
        WithTitleStyle(pterm.NewStyle(pterm.FgLightCyan)).
        WithShowCount(false).
        WithShowPercentage(true).
        WithTotal(totalSize).
        WithTitle(title).
        Start()

    buf := make([]byte, 32*1024)
    for {
        n, err := resp.Body.Read(buf)
        if n > 0 {
            _, _ = out.Write(buf[:n])
            pb.Add(n)
            
            currentMB := float64(pb.Current) / 1024 / 1024
            totalMB := float64(totalSize) / 1024 / 1024
            pb.UpdateTitle(fmt.Sprintf("%s: %.2f MB / %.2f MB", title, currentMB, totalMB))
        }
        if err == io.EOF { break }
        if err != nil { return err }
    }

    pb.UpdateTitle(pterm.LightGreen("Done: ") + title)
    pb.Stop()
    return nil
}

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
    libsDir := filepath.Join(appDir, "libs")

    if _, err := os.Stat(binPath); os.IsNotExist(err) {
        pterm.Error.Printf("- Executable binary '%s' not found at %s", meta.Exec, binPath)
        os.Exit(1)
    }

    rawEnv := os.Environ()
    var finalEnv []string

    ldPaths := []string{binDir, libsDir}
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

    finalEnv = append(finalEnv, "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1")

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

func getLocalFileInfo(path string) (string, string) {
    f, err := os.Open(path)
    if err != nil { return "", "" }
    defer f.Close()
    gz, _ := gzip.NewReader(f)
    tr := tar.NewReader(gz)
    for {
        header, err := tr.Next()
        if err == io.EOF { break }
        if filepath.Base(header.Name) == "metadata.json" {
            var m Metadata
            content, _ := io.ReadAll(tr)
            _ = json.Unmarshal(content, &m)
            return m.Name, m.Version
        }
    }
    return "", ""
}

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

func askConfirmation(message string) bool {
    if autoYes { return true }
    result, _ := pterm.DefaultInteractiveConfirm.WithDefaultText(message).Show()
    return result
}

func createDesktopShortcut(meta Metadata, appPath string) {
    matches, _ := filepath.Glob(filepath.Join(appPath, meta.Name+".*"))

    icon := ""
    if len(matches) > 0 {
        icon = matches[0]
    } else {
        icon = filepath.Join(appPath, "icon.png")
    }

    content := fmt.Sprintf("[Desktop Entry]\nName=%s\nExec=%s/%s\nIcon=%s\nType=Application\nCategories=Utility;\nComment=%s\nTerminal=false",
        meta.Name, BinPath, meta.Name, icon, meta.Description)

    _ = os.MkdirAll(Applications, 0755)
    _ = os.WriteFile(filepath.Join(Applications, "una-"+meta.Name+".desktop"), []byte(content), 0644)
}

func showHelp() {
    table := pterm.TableData{
        []string{"Command", "Description"},
        []string{"-i, install", "Install a package"},
        []string{"-u, update", "Update a package"},
        []string{"-r, remove", "Remove a package"},
        []string{"-l, list", "List installed packages"},
        []string{"-info", "Show package details"},
    }
    _ = pterm.DefaultTable.WithData(table).Render()
}

func handleAddRepo(args []string) {
    if os.Geteuid() != 0 {
        pterm.Error.Println("Root required.")
        return
    }
    if len(args) < 2 { return }
    url := strings.TrimSuffix(args[1], "/")
    sources := loadSources()
    sources = append(sources, RepoSource{URL: url, Branch: targetBranch})
    data, _ := json.MarshalIndent(sources, "", "  ")
    _ = os.WriteFile(SourcesFile, data, 0644)
    pterm.Success.Printf("+ Repository added: %s\n", url)
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
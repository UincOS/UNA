// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 Altha36. All rights reserved.

package main

import(
	"os"
	"io"
	"fmt"
	"strings"
	"net/http"
	"encoding/json"
	"github.com/pterm/pterm"
)

func FetchAllRemotePackages(appName string) (*RepoPackage, *RepoSource, error) {
    sources := loadSources()

    for _, source := range sources {
        branch := source.Branch
        if targetBranch != "stable" {
            branch = targetBranch
        }
        
        repoURL := fmt.Sprintf("%s/%s/packages.json", source.URL, branch)
        resp, err := http.Get(repoURL)
        if err != nil || resp.StatusCode != 200 {
            continue
        }
        
        var repo RepoSchema
        err = json.NewDecoder(resp.Body).Decode(&repo)
        resp.Body.Close()
        if err != nil {
            continue
        }

        allPackages := append(repo.Stable.Main, append(repo.Stable.Contrib, repo.Stable.NonFree...)...)
        
        for _, p := range allPackages {
            if strings.EqualFold(p.Name, appName) {
                selectedSource := source
                selectedSource.Branch = branch
                return &p, &selectedSource, nil
            }
        }
    }

    return nil, nil, fmt.Errorf("package '%s' not found in repositories", appName)
}

func downloadWithProgress(url, dest, title string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
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
        WithBarCharacter("━").
        WithLastCharacter(" ").
        WithBarFiller("─").
        WithShowCount(false).
        WithShowPercentage(true).
        WithTotal(totalSize).
        WithTitle(title).
        WithRemoveWhenDone(true).
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
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	pb.Stop()
	return nil
}
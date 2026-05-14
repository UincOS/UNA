// SPDX-License-Identifier: ASL-2.0
// Copyright (c) 2026 [Your Name/Entity]. All rights reserved.

package main

import (
	"fmt"
	"strconv"
    "crypto/sha256"
    "encoding/hex"
	"strings"
	"encoding/json"
	"path/filepath"
	"syscall"
    "io"
    "os"
)

func getActionName(isUpdate, force bool) string {
    if force { return "reinstallation" }
    if isUpdate { return "update" }
    return "installation"
}

func verifyChecksum(filePath string, expectedHash string) error {
	f, _ := os.Open(filePath)
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(expectedHash) {
		return fmt.Errorf("hash mismatch")
	}
	return nil
}

func acquireLock() error {
	if _, err := os.Stat(LockFile); err == nil { return fmt.Errorf("UNA is already running") }
	return os.WriteFile(LockFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func releaseLock() { _ = os.Remove(LockFile) }

func getVersionInstalled(appName string) string {
	files, err := os.ReadDir(StoreBase)
	if err != nil {
		return ""
	}

	realName := ""
	for _, f := range files {
		if strings.EqualFold(f.Name(), appName) {
			realName = f.Name()
			break
		}
	}

	if realName == "" {
		return ""
	}

	metaPath := filepath.Join(StoreBase, realName, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}
	var m Metadata
	_ = json.Unmarshal(data, &m)
	return m.Version
}

func setAppHash(path string, hash string) {
	_ = syscall.Setxattr(path, "user.una.hash", []byte(hash), 0)
}

func getAppHash(path string) string {
	buf := make([]byte, 64)
	sz, err := syscall.Getxattr(path, "user.una.hash", buf)
	if err != nil {
		return ""
	}
	return string(buf[:sz])
}

func isFileOk(path string, expectedSize int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() == expectedSize
}

func getActionVerb(isUpdate, force bool) string {
    if force { return "Fixing" }
    if isUpdate { return "Updating" }
    return "Installing"
}

func getActionState(isUpdate, force bool) string {
    if force { return "fixed" }
    if isUpdate { return "updated" }
    return "installed"
}
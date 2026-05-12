# 🌌 UNA Package Manager
> **© 2026 Altha36. Licensed under the Altha Project License (APL) v1.2.**

![UNA License](https://img.shields.io/badge/license-APL_v1.2-%23f9a761?style=for-the-badge)
![Go Version](https://img.shields.io/badge/go-1.24+-blue?style=for-the-badge&logo=go)
![Platform](https://img.shields.io/badge/platform-UincOS-lightgrey?style=for-the-badge&logo=linux)

**🚫 FORKING POLICY:** Forks of this project are **STRICTLY PROHIBITED**. This repository is provided for distribution, building, and evaluation purposes only. Any unauthorized copying, modification, or redistribution of this code is a violation of the Altha Project License (APL) v1.2.

---

**UNA** is a next-generation, high-performance package manager built from the ground up in Go. Developed as the core of the **Altha36** ecosystem and **UincOS**, UNA bridges the gap between the simplicity of Debian and the bleeding-edge speed of Arch Linux.

It is designed to be **atomic, isolated, and incredibly fast**, ensuring that your system stays clean while your applications run with native performance.

---

## 🚀 Key Features

### ⚡ Arch-Speed Stream Extraction
Unlike traditional managers that download, save, and then extract, UNA uses a **Stream-Based Engine**. Using `io.TeeReader`, it hashes and extracts packages in a single pass directly from the network to the disk, maximizing I/O efficiency.

### 🛡️ Atomic Security & Integrity
* **Atomic Move:** Installations are staged in a "shadow" directory. The application only becomes official once the integrity is verified and a final `os.Rename` is executed.
* **SHA-256 Validation:** Every byte is checked against a repository-provided hash to prevent corruption or tampering.
* **Write-Locking:** A specialized locking mechanism (`/tmp/una.lock`) prevents concurrent write operations, protecting your metadata from corruption.

### 📦 Superior Dependency Management
UNA solves the "Dependency Hell" without the bloat of Flatpaks or Snaps:
* **LD_LIBRARY_PATH Isolation:** Each app carries its own required libraries in a dedicated folder, injected at runtime.
* **System Integration:** Automatically links binaries to `/usr/local/bin` and generates `.desktop` entries for your application menu.

---

## 📂 Project Architecture

UNA is built with a modular approach for easy maintenance:

* **`core.go`**: The engine. Manages the lifecycle of a package (Fetch -> Verify -> Extract -> Finalize).
* **`utils.go`**: The toolbox. Contains system-level helpers, lock logic, and path resolution.
* **`types.go`**: The blueprint. Defines the internal schemas for repositories and package metadata.

---

## 🛠️ Installation

*Note: UNA is currently in active development. Use at your own risk.*

To compile UNA and install it on your system:

```bash
# Clone the repository (for personal build/use only)
git clone [https://github.com/Altha36/una.git](https://github.com/Altha36/una.git)
cd una

# Build the binary
go build -o una

# Install to system path
sudo mv una /usr/local/bin/ or sudo mv /usr/bin
```
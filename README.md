# 🌌 UNA Package Manager

![UNA License](https://img.shields.io/badge/license-MIT-orange-gold?style=for-the-badge)
![Go Version](https://img.shields.io/badge/go-1.21%2B-blue?style=for-the-badge&logo=go)
![Platform](https://img.shields.io/badge/platform-Debian%20%7C%20UincOS-lightgrey?style=for-the-badge&logo=linux)

**UNA** is a next-generation, high-performance package manager built from the ground up in Go. Originally developed for the **UincOS** ecosystem, UNA bridges the gap between the simplicity of Debian and the bleeding-edge speed of Arch Linux.

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

To compile UNA and install it on your system:

```bash
# Clone the repository
git clone [https://github.com/UincOS-Team/una.git](https://github.com/UincOS-Team/una.git)
cd una

# Build the binary
go build -o una

# Install to system path
sudo mv una /usr/local/bin/ or sudo mv una /usr/bin
```

###### In development, so this is the final readme but the tool ins't ready yet!

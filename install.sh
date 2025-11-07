#!/bin/bash
#author Semyon5700
# uir package manager installer
# Version: 1.0

set -e

echo "=== uir Package Manager Installer ==="
echo "Version: 1.0"
echo author Semyon5700

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run as root"
    exit 1
fi

# Check for Go compiler
if ! command -v go &> /dev/null; then
    echo "Error: Go compiler is required but not installed."
    echo "Please install Go from: https://golang.org/dl/"
    exit 1
fi

# Build uir
echo "Building uir package manager..."
go build -o uir main.go

# Build uir-build tool
echo "Building uir-build tool..."
cat > uir-build.go << 'EOF'
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PackageConfig struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Arch         string            `json:"arch"`
	InstallPaths map[string]string `json:"install_paths"`
	BinLinks     map[string]string `json:"bin_links"`
	Dependencies []string          `json:"dependencies"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: uir-build <config.json> <output.uir>")
		return
	}

	configFile := os.Args[1]
	outputFile := os.Args[2]

	if !strings.HasSuffix(outputFile, ".uir") {
		fmt.Println("Error: Output file must have .uir extension")
		return
	}

	// Read config
	config, err := readConfig(configFile)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		return
	}

	// Create package
	if err := createPackage(config, outputFile); err != nil {
		fmt.Printf("Package creation error: %v\n", err)
		return
	}

	fmt.Printf("Package created: %s\n", outputFile)
}

func readConfig(configFile string) (PackageConfig, error) {
	var config PackageConfig
	
	file, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}

	return config, nil
}

func createPackage(config PackageConfig, outputFile string) error {
	// Create tar.gz file
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add set.conf to archive
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: "set.conf",
		Size: int64(len(configData)),
		Mode: 0644,
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(configData); err != nil {
		return err
	}

	// Add files from install_paths
	for src := range config.InstallPaths {
		if err := addFileToArchive(tw, src); err != nil {
			return err
		}
	}

	return nil
}

func addFileToArchive(tw *tar.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = filepath.Base(filename)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}
EOF

go build -o uir-build uir-build.go

# Install binaries
echo "Installing binaries to /usr/local/bin/"
cp uir /usr/local/bin/
cp uir-build /usr/local/bin/

# Create directories
echo "Creating directory structure..."
mkdir -p /uir_packages
mkdir -p /etc/uir
mkdir -p /tmp/uir_temp

# Create uir package for self-management
echo "Creating uir self-management package..."
mkdir -p /uir_packages/uir
cp uir /uir_packages/uir/
cp uir-build /uir_packages/uir/

# Create package config for uir itself
cat > /uir_packages/uir/set.conf << EOF
{
  "name": "uir",
  "version": "1.0",
  "description": "Universal Package Manager",
  "arch": "any",
  "install_paths": {
    "uir": "/usr/local/bin/uir",
    "uir-build": "/usr/local/bin/uir-build"
  },
  "bin_links": {},
  "dependencies": []
}
EOF

# Register uir in package database
cat > /etc/uir/installed.json << EOF
{
  "uir": {
    "name": "uir",
    "version": "1.0",
    "install_date": "$(date +%Y-%m-%d)",
    "config": {
      "name": "uir",
      "version": "1.0",
      "description": "Universal Package Manager",
      "arch": "any",
      "install_paths": {
        "uir": "/usr/local/bin/uir",
        "uir-build": "/usr/local/bin/uir-build"
      },
      "bin_links": {},
      "dependencies": []
    }
  }
}
EOF

# Set permissions
chmod 755 /usr/local/bin/uir
chmod 755 /usr/local/bin/uir-build
chmod -R 755 /uir_packages
chmod -R 755 /etc/uir

# Cleanup
rm -f uir uir-build uir-build.go

echo ""
echo "=== Installation Complete ==="
echo "uir package manager v1.0 has been installed successfully!"
echo ""
echo "Available commands:"
echo "  uir install <package.uir>   - Install a package"
echo "  uir remove <package>        - Remove a package" 
echo "  uir list                    - List installed packages"
echo "  uir info <package>          - Show package info"
echo "  uir self-update <package>   - Update uir itself"
echo "  uir self-remove             - Completely remove uir"
echo "  uir version                 - Show version"
echo ""
echo "Package building:"
echo "  uir-build <config.json> <output.uir> - Create a package"
echo ""

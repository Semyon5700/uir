package main
 // author Semyon5700
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

type InstalledPackage struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	InstallDate string        `json:"install_date"`
	Config      PackageConfig `json:"config"`
}

const (
	UIR_DIR     = "/uir_packages"
	UIR_CONFIG  = "/etc/uir"
	UIR_TEMP    = "/tmp/uir_temp"
	UIR_VERSION = "1.0"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	
	switch command {
	case "install":
		if len(os.Args) < 3 {
			fmt.Println("Error: Please specify package file")
			return
		}
		installPackage(os.Args[2])
	case "remove":
		if len(os.Args) < 3 {
			fmt.Println("Error: Please specify package name")
			return
		}
		removePackage(os.Args[2])
	case "list":
		listPackages()
	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Error: Please specify package name")
			return
		}
		packageInfo(os.Args[2])
	case "update":
		updatePackageManager()
	case "self-update":
		if len(os.Args) < 3 {
			fmt.Println("Error: Please specify package file for update")
			return
		}
		selfUpdate(os.Args[2])
	case "version":
		fmt.Printf("uir package manager v%s\n", UIR_VERSION)
	case "self-remove":
		removeUirSelf()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Printf("uir package manager v%s\n", UIR_VERSION)
	fmt.Println("Usage: uir {install|remove|list|info|update|self-update|self-remove|version} [package]")
	fmt.Println("  install      - Install a package")
	fmt.Println("  remove       - Remove a package (including uir itself)")
	fmt.Println("  list         - List installed packages") 
	fmt.Println("  info         - Show package information")
	fmt.Println("  update       - Update package manager")
	fmt.Println("  self-update  - Update uir from .uir package")
	fmt.Println("  self-remove  - Completely remove uir package manager")
	fmt.Println("  version      - Show version")
}

func installPackage(pkgFile string) {
	if os.Geteuid() != 0 {
		fmt.Println("Error: Installation requires root privileges")
		return
	}

	if !strings.HasSuffix(pkgFile, ".uir") {
		fmt.Println("Error: Package file must have .uir extension")
		return
	}

	if _, err := os.Stat(pkgFile); os.IsNotExist(err) {
		fmt.Printf("Error: Package file %s does not exist\n", pkgFile)
		return
	}

	fmt.Printf("Installing package %s...\n", pkgFile)

	// Create temp directory
	extractDir := filepath.Join(UIR_TEMP, "install")
	os.RemoveAll(extractDir)
	os.MkdirAll(extractDir, 0755)
	defer os.RemoveAll(extractDir)

	// Extract archive
	if err := extractTarGz(pkgFile, extractDir); err != nil {
		fmt.Printf("Extraction error: %v\n", err)
		return
	}

	// Read config
	configFile := filepath.Join(extractDir, "set.conf")
	config, err := readConfig(configFile)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		return
	}

	// Check if package is already installed
	if isPackageInstalled(config.Name) {
		fmt.Printf("Package %s is already installed. Use 'uir remove %s' first.\n", config.Name, config.Name)
		return
	}

	// Create package directory
	pkgDir := filepath.Join(UIR_DIR, config.Name)
	os.RemoveAll(pkgDir)
	os.MkdirAll(pkgDir, 0755)

	// Copy package files
	if err := copyDir(extractDir, pkgDir); err != nil {
		fmt.Printf("Copy error: %v\n", err)
		return
	}

	// Install files
	if err := installFiles(config, pkgDir); err != nil {
		fmt.Printf("Installation error: %v\n", err)
		return
	}

	// Create bin links
	if err := createBinLinks(config, pkgDir); err != nil {
		fmt.Printf("Symlink error: %v\n", err)
		return
	}

	// Save installation info
	if err := saveInstallInfo(config); err != nil {
		fmt.Printf("Database error: %v\n", err)
		return
	}

	fmt.Printf("Package %s v%s successfully installed!\n", config.Name, config.Version)
}

func removePackage(pkgName string) {
	if os.Geteuid() != 0 {
		fmt.Println("Error: Removal requires root privileges")
		return
	}

	// Special case: remove uir itself
	if pkgName == "uir" {
		removeUirSelf()
		return
	}

	pkgDir := filepath.Join(UIR_DIR, pkgName)
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		fmt.Printf("Package %s is not installed\n", pkgName)
		return
	}

	fmt.Printf("Removing package %s...\n", pkgName)

	// Read config for file information
	configFile := filepath.Join(pkgDir, "set.conf")
	config, err := readConfig(configFile)
	if err == nil {
		// Remove installed files
		removeInstalledFiles(config)
		// Remove bin links
		removeBinLinks(config)
	}

	// Remove package directory
	os.RemoveAll(pkgDir)

	// Remove from database
	removeFromDatabase(pkgName)

	fmt.Printf("Package %s successfully removed!\n", pkgName)
}

func removeUirSelf() {
	if os.Geteuid() != 0 {
		fmt.Println("Error: Self-removal requires root privileges")
		return
	}

	fmt.Println("Removing uir package manager...")
	
	// Remove all installed packages first
	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	if file, err := os.Open(dbPath); err == nil {
		var packages map[string]InstalledPackage
		decoder := json.NewDecoder(file)
		if decoder.Decode(&packages) == nil {
			for pkgName := range packages {
				if pkgName != "uir" {
					fmt.Printf("Removing dependent package: %s\n", pkgName)
					pkgDir := filepath.Join(UIR_DIR, pkgName)
					os.RemoveAll(pkgDir)
				}
			}
		}
		file.Close()
	}

	// Remove uir package files
	uirDir := filepath.Join(UIR_DIR, "uir")
	os.RemoveAll(uirDir)

	// Remove system files and binaries
	os.Remove("/usr/local/bin/uir")
	os.Remove("/usr/local/bin/uir-build")
	os.RemoveAll(UIR_CONFIG)
	os.RemoveAll(UIR_DIR)
	os.RemoveAll(UIR_TEMP)

	fmt.Println("uir package manager completely removed!")
}

func listPackages() {
	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("No packages installed")
		return
	}

	file, err := os.Open(dbPath)
	if err != nil {
		fmt.Printf("Database error: %v\n", err)
		return
	}
	defer file.Close()

	var packages map[string]InstalledPackage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&packages); err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}

	fmt.Println("Installed packages:")
	fmt.Println("==================")
	for _, pkg := range packages {
		fmt.Printf("  %s (%s) - installed: %s\n", pkg.Name, pkg.Version, pkg.InstallDate)
	}
}

func packageInfo(pkgName string) {
	pkgDir := filepath.Join(UIR_DIR, pkgName)
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		fmt.Printf("Package %s is not installed\n", pkgName)
		return
	}

	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	file, err := os.Open(dbPath)
	if err != nil {
		fmt.Printf("Database error: %v\n", err)
		return
	}
	defer file.Close()

	var packages map[string]InstalledPackage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&packages); err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}

	pkg, exists := packages[pkgName]
	if !exists {
		fmt.Printf("Package %s not found in database\n", pkgName)
		return
	}

	fmt.Printf("Package Information: %s\n", pkg.Name)
	fmt.Println("====================")
	fmt.Printf("Version: %s\n", pkg.Version)
	fmt.Printf("Description: %s\n", pkg.Config.Description)
	fmt.Printf("Architecture: %s\n", pkg.Config.Arch)
	fmt.Printf("Install date: %s\n", pkg.InstallDate)
	
	if len(pkg.Config.InstallPaths) > 0 {
		fmt.Println("\nInstalled files:")
		for src, dest := range pkg.Config.InstallPaths {
			fmt.Printf("  %s -> %s\n", src, dest)
		}
	}
	
	if len(pkg.Config.BinLinks) > 0 {
		fmt.Println("\nBinary links:")
		for src, link := range pkg.Config.BinLinks {
			fmt.Printf("  %s -> %s\n", src, link)
		}
	}
}

func updatePackageManager() {
	fmt.Println("Checking for updates...")
	fmt.Println("Use: uir self-update <package.uir>")
}

func selfUpdate(pkgFile string) {
	if os.Geteuid() != 0 {
		fmt.Println("Error: Self-update requires root privileges")
		return
	}

	fmt.Println("Self-updating uir package manager...")
	
	// Use current uir to install the update
	cmd := exec.Command("/usr/local/bin/uir", "install", pkgFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Update error: %v\n", err)
		return
	}
	
	fmt.Println("uir package manager successfully updated to latest version!")
}

// Helper functions
func extractTarGz(src, dest string) error {
	cmd := exec.Command("tar", "-xzf", src, "-C", dest)
	return cmd.Run()
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

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func installFiles(config PackageConfig, pkgDir string) error {
	for src, dest := range config.InstallPaths {
		srcPath := filepath.Join(pkgDir, src)
		destPath := dest

		// Create target directory
		os.MkdirAll(filepath.Dir(destPath), 0755)

		if err := copyFile(srcPath, destPath); err != nil {
			return err
		}

		// Make executable for bin files
		if strings.HasPrefix(dest, "/usr/local/bin/") || 
		   strings.HasPrefix(dest, "/usr/bin/") ||
		   strings.HasPrefix(dest, "/bin/") {
			os.Chmod(destPath, 0755)
			fmt.Printf("Installed (executable): %s -> %s\n", src, dest)
		} else {
			fmt.Printf("Installed: %s -> %s\n", src, dest)
		}
	}
	return nil
}

func createBinLinks(config PackageConfig, pkgDir string) error {
	for src, linkName := range config.BinLinks {
		srcPath := filepath.Join(pkgDir, src)
		linkPath := filepath.Join("/usr/local/bin", linkName)

		// Ensure source file exists and is executable
		if _, err := os.Stat(srcPath); err == nil {
			os.Chmod(srcPath, 0755)
		}

		// Remove old symlink if exists
		os.Remove(linkPath)

		// Create new symlink
		if err := os.Symlink(srcPath, linkPath); err != nil {
			return err
		}

		fmt.Printf("Created binary link: %s -> %s\n", linkName, srcPath)
	}
	return nil
}

func removeInstalledFiles(config PackageConfig) {
	for _, dest := range config.InstallPaths {
		os.Remove(dest)
		fmt.Printf("Removed: %s\n", dest)
	}
}

func removeBinLinks(config PackageConfig) {
	for _, linkName := range config.BinLinks {
		linkPath := filepath.Join("/usr/local/bin", linkName)
		os.Remove(linkPath)
		fmt.Printf("Removed binary link: %s\n", linkName)
	}
}

func saveInstallInfo(config PackageConfig) error {
	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	
	var packages map[string]InstalledPackage
	
	// Read existing database
	if file, err := os.Open(dbPath); err == nil {
		decoder := json.NewDecoder(file)
		decoder.Decode(&packages)
		file.Close()
	}
	
	if packages == nil {
		packages = make(map[string]InstalledPackage)
	}
	
	packages[config.Name] = InstalledPackage{
		Name:        config.Name,
		Version:     config.Version,
		InstallDate: getCurrentDate(),
		Config:      config,
	}
	
	// Ensure config directory exists
	os.MkdirAll(UIR_CONFIG, 0755)
	
	file, err := os.Create(dbPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(packages)
}

func removeFromDatabase(pkgName string) error {
	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	
	file, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var packages map[string]InstalledPackage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&packages); err != nil {
		return err
	}

	delete(packages, pkgName)

	file, err = os.Create(dbPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(packages)
}

func isPackageInstalled(pkgName string) bool {
	dbPath := filepath.Join(UIR_CONFIG, "installed.json")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false
	}

	file, err := os.Open(dbPath)
	if err != nil {
		return false
	}
	defer file.Close()

	var packages map[string]InstalledPackage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&packages); err != nil {
		return false
	}

	_, exists := packages[pkgName]
	return exists
}

func getCurrentDate() string {
	cmd := exec.Command("date", "+%Y-%m-%d")
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output))
}

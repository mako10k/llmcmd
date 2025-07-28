package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	InstallDir     = "/usr/local/bin"
	BinaryName     = "llmcmd"
	GitHubRepo     = "mako10k/llmcmd"
	ConfigDir      = ".config/llmcmd"
	DefaultConfig  = `{
  "model": "gpt-4o-mini",
  "max_tokens": 4096,
  "temperature": 0.1,
  "max_api_calls": 50,
  "timeout_seconds": 300
}`
)

// SystemInstaller handles system-wide installation
type SystemInstaller struct {
	verbose bool
}

// NewSystemInstaller creates a new system installer
func NewSystemInstaller(verbose bool) *SystemInstaller {
	return &SystemInstaller{verbose: verbose}
}

// Install performs system-wide installation
func (si *SystemInstaller) Install() error {
	si.log("Starting llmcmd system installation...")

	// Check if running as root
	if err := si.checkPermissions(); err != nil {
		return err
	}

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Install binary
	targetPath := filepath.Join(InstallDir, BinaryName)
	if err := si.installBinary(currentExe, targetPath); err != nil {
		return err
	}

	// Create default config
	if err := si.createDefaultConfig(); err != nil {
		si.log("Warning: Failed to create default config: %v", err)
	}

	// Verify installation
	if err := si.verifyInstallation(); err != nil {
		return err
	}

	si.log("Installation completed successfully!")
	si.printUsageInfo()
	return nil
}

// checkPermissions verifies installation permissions
func (si *SystemInstaller) checkPermissions() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("system installation requires root privileges. Please run with sudo")
	}
	return nil
}

// installBinary copies the current binary to system location
func (si *SystemInstaller) installBinary(source, target string) error {
	si.log("Installing binary to %s...", target)

	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Copy binary
	if err := si.copyFile(source, target); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(target, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

// copyFile copies a file from source to destination
func (si *SystemInstaller) copyFile(source, dest string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// createDefaultConfig creates default configuration in user's home
func (si *SystemInstaller) createDefaultConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDirPath := filepath.Join(homeDir, ConfigDir)
	configFilePath := filepath.Join(configDirPath, "config.json")

	// Check if config already exists
	if _, err := os.Stat(configFilePath); err == nil {
		si.log("Config file already exists: %s", configFilePath)
		return nil
	}

	// Create config directory
	if err := os.MkdirAll(configDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config
	if err := os.WriteFile(configFilePath, []byte(DefaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	si.log("Created default config: %s", configFilePath)
	return nil
}

// verifyInstallation checks if installation was successful
func (si *SystemInstaller) verifyInstallation() error {
	si.log("Verifying installation...")

	// Check if binary exists in PATH
	_, err := exec.LookPath(BinaryName)
	if err != nil {
		return fmt.Errorf("installation verification failed: %s not found in PATH", BinaryName)
	}

	si.log("Installation verified successfully!")
	return nil
}

// printUsageInfo displays usage information
func (si *SystemInstaller) printUsageInfo() {
	fmt.Println()
	fmt.Println("🎉 llmcmd has been installed successfully!")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s 'your task description'\n", BinaryName)
	fmt.Printf("  echo 'data' | %s 'process this data'\n", BinaryName)
	fmt.Printf("  %s -i input.txt 'analyze this file'\n", BinaryName)
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  OPENAI_API_KEY=your_api_key")
	fmt.Println("  LLMCMD_MODEL=gpt-4o-mini")
	fmt.Println()
	fmt.Printf("For more help: %s --help\n", BinaryName)
}

// log prints log messages if verbose mode is enabled
func (si *SystemInstaller) log(format string, args ...interface{}) {
	if si.verbose {
		fmt.Printf("[INSTALL] "+format+"\n", args...)
	}
}

// GetPlatformInfo returns current platform information
func GetPlatformInfo() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// Uninstall removes the installed binary
func (si *SystemInstaller) Uninstall() error {
	if err := si.checkPermissions(); err != nil {
		return err
	}

	targetPath := filepath.Join(InstallDir, BinaryName)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("%s is not installed in %s", BinaryName, InstallDir)
	}

	si.log("Removing %s...", targetPath)
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	si.log("llmcmd uninstalled successfully!")
	return nil
}

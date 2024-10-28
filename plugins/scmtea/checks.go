package main

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ssotops/gitspace-plugin-sdk/logger"
)

// Check if Docker is running and accessible
func checkDockerRunning() error {
    cmd := exec.Command("docker", "info")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("docker is not running: %v (output: %s)", err, string(output))
    }
    return nil
}

// Check if required ports are available
func checkRequiredPorts() error {
    portsToCheck := []int{3000, 22} // Gitea web and SSH ports
    for _, port := range portsToCheck {
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
        if err == nil {
            conn.Close()
            return fmt.Errorf("port %d is already in use", port)
        }
    }
    return nil
}

// Check if Node.js is installed and meets version requirements
func checkNodeJS() error {
    // Check if Node.js is installed
    nodePath, err := exec.LookPath("node")
    if err != nil {
        return fmt.Errorf("Node.js not found in PATH")
    }

    // Check Node.js version
    cmd := exec.Command(nodePath, "--version")
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("failed to get Node.js version: %v", err)
    }

    version := strings.TrimSpace(string(output))
    // Remove the 'v' prefix if present
    version = strings.TrimPrefix(version, "v")

    // Parse version and check minimum requirements
    parts := strings.Split(version, ".")
    if len(parts) < 1 {
        return fmt.Errorf("invalid Node.js version format: %s", version)
    }

    major, err := strconv.Atoi(parts[0])
    if err != nil {
        return fmt.Errorf("invalid Node.js version number: %s", version)
    }

    if major < 14 {
        return fmt.Errorf("Node.js version 14 or higher is required, found version %s", version)
    }

    return nil
}

// Check if Bun is installed for running scripts
func checkBun() error {
    _, err := exec.LookPath("bun")
    if err != nil {
        return fmt.Errorf("Bun not found in PATH. Please install Bun: https://bun.sh")
    }
    return nil
}

// Check if Git is installed and configured
func checkGit() error {
    // Check Git installation
    _, err := exec.LookPath("git")
    if err != nil {
        return fmt.Errorf("Git not found in PATH")
    }

    // Check Git configuration
    configTests := []struct {
        name string
        args []string
    }{
        {"user.name", []string{"config", "--global", "--get", "user.name"}},
        {"user.email", []string{"config", "--global", "--get", "user.email"}},
    }

    for _, test := range configTests {
        cmd := exec.Command("git", test.args...)
        if output, err := cmd.Output(); err != nil || len(output) == 0 {
            return fmt.Errorf("Git %s is not configured", test.name)
        }
    }

    return nil
}

// Check system memory availability
func checkMemory() error {
    cmd := exec.Command("free", "-m")
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("failed to check system memory: %v", err)
    }

    // Parse free command output
    lines := strings.Split(string(output), "\n")
    if len(lines) < 2 {
        return fmt.Errorf("unexpected free command output format")
    }

    fields := strings.Fields(lines[1])
    if len(fields) < 4 {
        return fmt.Errorf("unexpected free command output format")
    }

    available, err := strconv.Atoi(fields[3])
    if err != nil {
        return fmt.Errorf("failed to parse available memory: %v", err)
    }

    if available < 512 { // Require at least 512MB available
        return fmt.Errorf("insufficient memory available: %dMB (minimum 512MB required)", available)
    }

    return nil
}

// Check disk space availability
func checkDiskSpace(path string) error {
    cmd := exec.Command("df", "-m", path)
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("failed to check disk space: %v", err)
    }

    lines := strings.Split(string(output), "\n")
    if len(lines) < 2 {
        return fmt.Errorf("unexpected df command output format")
    }

    fields := strings.Fields(lines[1])
    if len(fields) < 4 {
        return fmt.Errorf("unexpected df command output format")
    }

    available, err := strconv.Atoi(fields[3])
    if err != nil {
        return fmt.Errorf("failed to parse available disk space: %v", err)
    }

    if available < 1024 { // Require at least 1GB available
        return fmt.Errorf("insufficient disk space available: %dMB (minimum 1024MB required)", available)
    }

    return nil
}

// Check network connectivity
func checkNetwork(logger *logger.RateLimitedLogger) error {
    // Test DNS resolution
    _, err := net.LookupHost("github.com")
    if err != nil {
        return fmt.Errorf("DNS resolution failed: %v", err)
    }

    // Test HTTP connectivity
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Get("https://github.com")
    if err != nil {
        return fmt.Errorf("HTTP connectivity test failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP connectivity test failed with status: %d", resp.StatusCode)
    }

    return nil
}

// Comprehensive system check
func (p *ScmteaPlugin) checkPrerequisites() error {
    checks := []struct {
        name string
        fn   func() error
    }{
        {"Docker", checkDockerRunning},
        {"Ports", checkRequiredPorts},
        {"Node.js", checkNodeJS},
        {"Bun", checkBun},
        {"Git", checkGit},
        {"Memory", checkMemory},
        {"Network", func() error { return checkNetwork(p.logger) }},
    }

    for _, check := range checks {
        p.logger.Info(fmt.Sprintf("Checking %s...", check.name))
        if err := check.fn(); err != nil {
            p.logger.Error(fmt.Sprintf("%s check failed", check.name), "error", err)
            return fmt.Errorf("%s check failed: %w", check.name, err)
        }
        p.logger.Info(fmt.Sprintf("%s check passed", check.name))
    }

    // Check disk space last as we now know the install path
    installPath, err := getComposePath()
    if err != nil {
        return fmt.Errorf("failed to get installation path: %w", err)
    }
    
    if err := checkDiskSpace(filepath.Dir(installPath)); err != nil {
        p.logger.Error("Disk space check failed", "error", err)
        return fmt.Errorf("disk space check failed: %w", err)
    }

    return nil
}

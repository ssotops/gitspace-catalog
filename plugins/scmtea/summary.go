// summary.go

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

func printGiteaSummary(logger *logger.RateLimitedLogger) (string, error) {
	logger.Debug("Starting printGiteaSummary function")

	runDockerCommand := func(name string, args ...string) (string, error) {
		cmd := exec.Command("docker", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("Docker command failed",
				"command", name,
				"args", args,
				"error", err,
				"output", string(output))
			return "", fmt.Errorf("%s failed: %v - Output: %s", name, err, output)
		}
		logger.Debug("Docker command succeeded",
			"command", name,
			"output", string(output))
		return strings.TrimSpace(string(output)), nil
	}

	_, err := runDockerCommand("Check Docker", "version")
	if err != nil {
		return "", fmt.Errorf("Docker daemon is not accessible: %v", err)
	}

	allContainers, err := runDockerCommand("List containers", "ps", "--format", "{{.Names}}")
	if err != nil {
		return "", fmt.Errorf("Failed to list containers: %v", err)
	}
	logger.Debug("All running containers", "containers", allContainers)

	giteaContainer := ""
	dbContainer := ""
	for _, container := range strings.Split(allContainers, "\n") {
		if strings.Contains(container, "gitea") && !strings.Contains(container, "db") {
			giteaContainer = container
		} else if strings.Contains(container, "gitea") && strings.Contains(container, "db") {
			dbContainer = container
		}
	}

	if giteaContainer == "" || dbContainer == "" {
		logger.Warn("Gitea containers are not running")
		return "", fmt.Errorf("Gitea containers are not running. Please start Gitea first.")
	}

	giteaPort, err := runDockerCommand("Get Gitea port", "port", giteaContainer, "3000")
	if err != nil {
		logger.Warn("Failed to get Gitea port, using default", "error", err)
		giteaPort = "3000"
	}

	giteaSshPort, err := runDockerCommand("Get Gitea SSH port", "port", giteaContainer, "22")
	if err != nil {
		logger.Warn("Failed to get Gitea SSH port, using default", "error", err)
		giteaSshPort = "22"
	}

	giteaIp, err := runDockerCommand("Get Gitea IP", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", giteaContainer)
	if err != nil {
		logger.Warn("Failed to get Gitea IP", "error", err)
		giteaIp = "N/A"
	}

	summary := fmt.Sprintf(`Gitea Summary:
Gitea Container:
  Name: %s
  Web UI: http://localhost:%s
  SSH: ssh://localhost:%s
  Internal IP: %s

Database Container:
  Name: %s
  Internal Port: 5432
  Internal IP: %s`,
		giteaContainer,
		strings.TrimPrefix(giteaPort, "0.0.0.0:"),
		strings.TrimPrefix(giteaSshPort, "0.0.0.0:"),
		giteaIp,
		dbContainer,
		giteaIp)

	return summary, nil
}

func gitConfigSummary() (*pb.CommandResponse, error) {
	getGitConfig := func(scope string) (string, error) {
		name, _ := exec.Command("git", "config", "--"+scope, "--get", "user.name").Output()
		email, _ := exec.Command("git", "config", "--"+scope, "--get", "user.email").Output()
		return fmt.Sprintf("Name: %s\nEmail: %s",
			strings.TrimSpace(string(name)),
			strings.TrimSpace(string(email))), nil
	}

	globalConfig, err := getGitConfig("global")
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error getting global git config: %v", err),
		}, nil
	}

	summary := fmt.Sprintf("Global Git Config:\n%s\n\n", globalConfig)

	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err == nil {
		localConfig, err := getGitConfig("local")
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Error getting local git config: %v", err),
			}, nil
		}
		pwd, _ := os.Getwd()
		summary += fmt.Sprintf("Local Git Config (%s):\n%s",
			filepath.Base(pwd), localConfig)
	} else {
		summary += "Local Git Config:\nNot a Git repository"
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  summary,
	}, nil
}

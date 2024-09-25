package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"google.golang.org/protobuf/proto"
)

type ScmteaPlugin struct{}

func (p *ScmteaPlugin) GetPluginInfo(req *pb.PluginInfoRequest) (*pb.PluginInfo, error) {
	log.Info("GetPluginInfo called")
	return &pb.PluginInfo{
		Name:    "Scmtea Plugin",
		Version: "1.0.0",
	}, nil
}

func (p *ScmteaPlugin) ExecuteCommand(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	switch req.Command {
	case "start":
		return runDockerCompose("up -d")
	case "stop":
		return runDockerCompose("down")
	case "restart":
		return runDockerCompose("restart")
	case "force_recreate":
		return runDockerCompose("up -d --force-recreate")
	case "print_summary":
		return printGiteaSummary()
	case "delete_containers_images":
		return deleteContainersAndImages()
	case "delete_volumes":
		return deleteVolumes()
	case "setup":
		return setupGitea(req)
	case "git_config_summary":
		return gitConfigSummary()
	default:
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Unknown command",
		}, nil
	}
}

func (p *ScmteaPlugin) GetMenu(req *pb.MenuRequest) (*pb.MenuResponse, error) {
	menuOptions := []gsplug.MenuOption{
		{
			Label:   "Setup Gitea",
			Command: "setup",
			Parameters: []gsplug.ParameterInfo{
				{Name: "username", Description: "Gitea username", Required: true},
				{Name: "password", Description: "Gitea password", Required: true},
				{Name: "email", Description: "Gitea email", Required: true},
				{Name: "git_name", Description: "Name for Git commits", Required: true},
				{Name: "repo_name", Description: "Repository name", Required: true},
				{Name: "ssh_port", Description: "SSH port for Gitea (default is 22)", Required: false},
			},
		},
		{Label: "Start Gitea", Command: "start"},
		{Label: "Stop Gitea", Command: "stop"},
		{Label: "Restart Gitea", Command: "restart"},
		{Label: "Force Recreate Gitea", Command: "force_recreate"},
		{Label: "Print Gitea Summary", Command: "print_summary"},
		{Label: "Print Git Config Summary", Command: "git_config_summary"},
		{Label: "Delete Gitea Containers and Images", Command: "delete_containers_images"},
		{Label: "Delete Volumes", Command: "delete_volumes"},
	}

	menuBytes, err := json.Marshal(menuOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal menu: %w", err)
	}

	return &pb.MenuResponse{
		MenuData: menuBytes,
	}, nil
}

func runDockerCompose(args ...string) (*pb.CommandResponse, error) {
	cmd := exec.Command("docker-compose", append([]string{"-f", "/Users/alechp/Code/alechp/.repositories/config-backup/scripts/gitea/docker-compose.yaml"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error: %v, Output: %s", err, output),
		}, nil
	}
	return &pb.CommandResponse{
		Success: true,
		Result:  string(output),
	}, nil
}

func printGiteaSummary() (*pb.CommandResponse, error) {
	giteaContainer, err := exec.Command("docker", "ps", "--filter", "name=gitea", "--format", "{{.Names}}").Output()
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error getting Gitea container: %v", err)}, nil
	}

	dbContainer, err := exec.Command("docker", "ps", "--filter", "name=gitea_db", "--format", "{{.Names}}").Output()
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error getting DB container: %v", err)}, nil
	}

	if len(giteaContainer) == 0 || len(dbContainer) == 0 {
		return &pb.CommandResponse{Success: false, ErrorMessage: "Gitea containers are not running. Please start Gitea first."}, nil
	}

	giteaPort, _ := exec.Command("docker", "port", strings.TrimSpace(string(giteaContainer)), "3000").Output()
	giteaSshPort, _ := exec.Command("docker", "port", strings.TrimSpace(string(giteaContainer)), "22").Output()
	giteaIp, _ := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", strings.TrimSpace(string(giteaContainer))).Output()

	dbPort, _ := exec.Command("docker", "port", strings.TrimSpace(string(dbContainer)), "5432").Output()
	dbIp, _ := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", strings.TrimSpace(string(dbContainer))).Output()

	summary := fmt.Sprintf(`Gitea Summary:
Gitea Container:
  Name: %s
  Web UI: http://localhost:%s
  SSH: ssh://localhost:%s
  Internal IP: %s

Database Container:
  Name: %s
  Port: %s
  Internal IP: %s`,
		strings.TrimSpace(string(giteaContainer)),
		strings.TrimSpace(strings.Split(string(giteaPort), ":")[1]),
		strings.TrimSpace(strings.Split(string(giteaSshPort), ":")[1]),
		strings.TrimSpace(string(giteaIp)),
		strings.TrimSpace(string(dbContainer)),
		strings.TrimSpace(strings.Split(string(dbPort), ":")[1]),
		strings.TrimSpace(string(dbIp)))

	return &pb.CommandResponse{
		Success: true,
		Result:  summary,
	}, nil
}

func deleteContainersAndImages() (*pb.CommandResponse, error) {
	_, err := runDockerCompose("down")
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error stopping containers: %v", err)}, nil
	}

	cmd := exec.Command("docker", "rmi", "$(docker images 'gitea/gitea' -q)")
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error removing Gitea images: %v", err)}, nil
	}

	cmd = exec.Command("docker", "rmi", "$(docker images 'postgres:13' -q)")
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error removing Postgres images: %v", err)}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Containers and images have been deleted.",
	}, nil
}

func deleteVolumes() (*pb.CommandResponse, error) {
	_, err := runDockerCompose("down", "-v")
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error deleting volumes: %v", err)}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Volumes have been deleted.",
	}, nil
}

func gitConfigSummary() (*pb.CommandResponse, error) {
	getGitConfig := func(scope string) (string, error) {
		name, _ := exec.Command("git", "config", "--"+scope, "--get", "user.name").Output()
		email, _ := exec.Command("git", "config", "--"+scope, "--get", "user.email").Output()
		return fmt.Sprintf("Name: %s\nEmail: %s", strings.TrimSpace(string(name)), strings.TrimSpace(string(email))), nil
	}

	globalConfig, err := getGitConfig("global")
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error getting global git config: %v", err)}, nil
	}

	summary := fmt.Sprintf("Global Git Config:\n%s\n\n", globalConfig)

	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err == nil {
		localConfig, err := getGitConfig("local")
		if err != nil {
			return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error getting local git config: %v", err)}, nil
		}
		pwd, _ := os.Getwd()
		summary += fmt.Sprintf("Local Git Config (%s):\n%s", filepath.Base(pwd), localConfig)
	} else {
		summary += "Local Git Config:\nNot a Git repository"
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  summary,
	}, nil
}

func main() {
	logDir := filepath.Join("logs", "scmtea")
	logger, err := logger.NewRateLimitedLogger(logDir, "scmtea")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Scmtea plugin starting")

	plugin := &ScmteaPlugin{}

	for {
		logger.Debug("Waiting for message")
		msgType, msg, err := gsplug.ReadMessage(os.Stdin)
		if err != nil {
			if err == io.EOF {
				logger.Info("Received EOF, exiting")
				return
			}
			logger.Error("Error reading message", "error", err)
			continue
		}
		logger.Debug("Received message", "type", msgType, "content", fmt.Sprintf("%+v", msg))

		var response proto.Message
		switch msgType {
		case 1: // GetPluginInfo
			response, err = plugin.GetPluginInfo(msg.(*pb.PluginInfoRequest))
		case 2: // ExecuteCommand
			response, err = plugin.ExecuteCommand(msg.(*pb.CommandRequest))
		case 3: // GetMenu
			response, err = plugin.GetMenu(msg.(*pb.MenuRequest))
		default:
			err = fmt.Errorf("unknown message type: %d", msgType)
		}

		if err != nil {
			logger.Error("Error handling message", "error", err)
			continue
		}

		logger.Debug("Sending response", "type", msgType, "content", fmt.Sprintf("%+v", response))
		err = gsplug.WriteMessage(os.Stdout, response)
		if err != nil {
			logger.Error("Error writing response", "error", err)
		} else {
			logger.Debug("Response sent successfully")
		}

		os.Stdout.Sync()
	}
}

func setupGitea(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	username := req.Parameters["username"]
	password := req.Parameters["password"]
	email := req.Parameters["email"]
	gitName := req.Parameters["git_name"]
	repoName := req.Parameters["repo_name"]
	sshPort := req.Parameters["ssh_port"]
	if sshPort == "" {
		sshPort = "22"
	}

	// Generate SSH key
	sshKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519_gitea")
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", sshKeyPath, "-N", "")
	if err := cmd.Run(); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error generating SSH key: %v", err)}, nil
	}

	// Add SSH config
	configPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	configContent := fmt.Sprintf(`
Host gitea-local
    HostName localhost
    Port %s
    User git
    IdentityFile %s
`, sshPort, sshKeyPath)
	if err := appendToFile(configPath, configContent); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error updating SSH config: %v", err)}, nil
	}

	// Upload SSH key to Gitea
	sshKey, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error reading SSH key: %v", err)}, nil
	}
	keyTitle := "Automated SSH Key"
	if err := uploadSSHKey(username, password, string(sshKey), keyTitle); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error uploading SSH key: %v", err)}, nil
	}

	// Clone the repository
	if err := cloneRepo(repoName, username); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error cloning repository: %v", err)}, nil
	}

	// Set local Git configuration
	if err := setGitConfig(repoName, gitName, email); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error setting Git config: %v", err)}, nil
	}

	// Update remote URL
	if err := updateRemoteURL(repoName, username); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error updating remote URL: %v", err)}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Gitea setup completed successfully!",
	}, nil
}

func appendToFile(filename, content string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func uploadSSHKey(username, password, sshKey, keyTitle string) error {
	url := "http://localhost:3000/api/v1/user/keys"
	payload := fmt.Sprintf(`{"title":"%s","key":"%s"}`, keyTitle, sshKey)
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to upload SSH key, status: %s", resp.Status)
	}
	return nil
}

func cloneRepo(repoName, username string) error {
	cmd := exec.Command("git", "clone", fmt.Sprintf("http://localhost:3000/%s/%s.git", username, repoName))
	return cmd.Run()
}

func setGitConfig(repoName, gitName, email string) error {
	cmd := exec.Command("git", "-C", repoName, "config", "user.name", gitName)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "-C", repoName, "config", "user.email", email)
	return cmd.Run()
}

func updateRemoteURL(repoName, username string) error {
	cmd := exec.Command("git", "-C", repoName, "remote", "set-url", "origin", fmt.Sprintf("git@gitea-local:%s/%s.git", username, repoName))
	return cmd.Run()
}

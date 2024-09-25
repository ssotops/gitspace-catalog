package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"google.golang.org/protobuf/proto"
)

//go:embed default-docker-compose.yaml
var defaultComposeFile embed.FS

const (
	pluginDataDir          = "/.ssot/gitspace/plugins/data/scmtea"
	composeFileName        = "docker-compose.yaml"
	defaultComposeFileName = "default-docker-compose.yaml"
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
	case "set_compose_file":
		return setComposeFile(req.Parameters["option"], req.Parameters["custom_path"])
	case "start":
		return runDockerCompose("up", "-d")
	case "stop":
		return runDockerCompose("down")
	case "restart":
		return runDockerCompose("restart")
	case "force_recreate":
		return runDockerCompose("up", "-d", "--force-recreate")
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
			Label:   "Set Docker Compose File",
			Command: "set_compose_file",
			Parameters: []gsplug.ParameterInfo{
				{Name: "option", Description: "Choose Docker Compose file option (Use default, Enter custom path, Go back)", Required: true},
				{Name: "custom_path", Description: "Path to custom docker-compose.yaml file", Required: false},
			},
		},
		{
			Label:   "Setup Gitea",
			Command: "setup",
			Parameters: []gsplug.ParameterInfo{
				{Name: "username", Description: "Gitea username", Required: true},
				{Name: "password", Description: "Gitea password (will not be displayed)", Required: true},
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

func setComposeFile(option, customPath string) (*pb.CommandResponse, error) {
	dataDir := filepath.Join(os.Getenv("HOME"), pluginDataDir)
	destPath := filepath.Join(dataDir, composeFileName)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create plugin data directory: %v", err),
		}, nil
	}

	switch option {
	case "Use default":
		defaultCompose, err := defaultComposeFile.ReadFile(defaultComposeFileName)
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read default docker-compose.yaml: %v", err),
			}, nil
		}
		if err = ioutil.WriteFile(destPath, defaultCompose, 0644); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to write default docker-compose.yaml: %v", err),
			}, nil
		}
	case "Enter custom path":
		if customPath == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required when choosing to enter a custom path",
			}, nil
		}
		if _, err := os.Stat(customPath); os.IsNotExist(err) {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("The specified docker-compose.yaml file does not exist: %s", customPath),
			}, nil
		}
		input, err := ioutil.ReadFile(customPath)
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read custom docker-compose.yaml: %v", err),
			}, nil
		}
		if err = ioutil.WriteFile(destPath, input, 0644); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to copy custom docker-compose.yaml: %v", err),
			}, nil
		}
	case "Go back":
		return &pb.CommandResponse{
			Success: true,
			Result:  "Operation cancelled",
		}, nil
	default:
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Invalid option selected",
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("Docker Compose file successfully set and copied to %s", destPath),
	}, nil
}

func runDockerCompose(args ...string) (*pb.CommandResponse, error) {
	log.Info("Running docker-compose command", "args", args)

	composePath, err := getComposePath()
	if err != nil {
		log.Error("Error getting compose path", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	log.Info("Compose file path", "path", composePath)

	// Try docker-compose command
	log.Info("Attempting to run docker-compose command")
	cmd := exec.Command("docker-compose", append([]string{"-f", composePath}, args...)...)
	output, err := cmd.CombinedOutput()
	log.Info("docker-compose command output", "output", string(output))

	if err != nil {
		log.Error("docker-compose command failed, attempting docker compose", "error", err)
		// If docker-compose fails, try docker compose
		cmd = exec.Command("docker", append([]string{"compose", "-f", composePath}, args...)...)
		output, err = cmd.CombinedOutput()
		log.Info("docker compose command output", "output", string(output))
	}

	if err != nil {
		log.Error("Error executing Docker Compose command", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error executing Docker Compose command: %v\nOutput: %s", err, string(output)),
		}, nil
	}

	log.Info("Docker Compose command executed successfully")
	return &pb.CommandResponse{
		Success: true,
		Result:  string(output),
	}, nil
}

func getComposePath() (string, error) {
	dataDir := filepath.Join(os.Getenv("HOME"), pluginDataDir)
	composePath := filepath.Join(dataDir, composeFileName)
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return "", fmt.Errorf("docker-compose.yaml not found. Please use 'Set Docker Compose File' to set it")
	}
	return composePath, nil
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
	log.Info("Starting deleteContainersAndImages")

	// Stop and remove containers
	log.Info("Stopping and removing containers with docker-compose down")
	downOutput, err := runDockerCompose("down")
	if err != nil {
		log.Error("Error stopping containers", "error", err, "output", downOutput)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error stopping containers: %v\nOutput: %s", err, downOutput)}, nil
	}
	log.Info("docker-compose down completed successfully")

	// Remove Gitea images
	log.Info("Attempting to remove Gitea images")
	giteaImages, err := exec.Command("docker", "images", "gitea/gitea", "-q").Output()
	if err != nil {
		log.Error("Error listing Gitea images", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error listing Gitea images: %v", err)}, nil
	}

	log.Info("Gitea images found", "images", string(giteaImages))
	if len(giteaImages) > 0 {
		cmd := exec.Command("docker", "rmi", "-f", strings.TrimSpace(string(giteaImages)))
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Error("Error removing Gitea images", "error", err, "output", string(output))
			return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error removing Gitea images: %v\nOutput: %s", err, output)}, nil
		}
		log.Info("Gitea images removed successfully")
	} else {
		log.Info("No Gitea images found to remove")
	}

	// Remove Postgres images
	log.Info("Attempting to remove Postgres images")
	postgresImages, err := exec.Command("docker", "images", "postgres:13", "-q").Output()
	if err != nil {
		log.Error("Error listing Postgres images", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error listing Postgres images: %v", err)}, nil
	}

	log.Info("Postgres images found", "images", string(postgresImages))
	if len(postgresImages) > 0 {
		cmd := exec.Command("docker", "rmi", "-f", strings.TrimSpace(string(postgresImages)))
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Error("Error removing Postgres images", "error", err, "output", string(output))
			return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error removing Postgres images: %v\nOutput: %s", err, output)}, nil
		}
		log.Info("Postgres images removed successfully")
	} else {
		log.Info("No Postgres images found to remove")
	}

	log.Info("deleteContainersAndImages completed successfully")
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

func setupGitea(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	// Start Gitea containers
	log.Info("Starting Gitea containers...")
	startResponse, err := runDockerCompose("up", "-d")
	if err != nil {
		log.Error("Failed to start Gitea containers", "error", err)
		return startResponse, err
	}
	log.Info("Gitea containers started successfully")

	// Wait for Gitea to be ready
	log.Info("Waiting for Gitea to be ready...")
	if err := waitForGitea(); err != nil {

		log.Error("Gitea failed to start within the expected time", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error waiting for Gitea to start: %v", err),
		}, nil
	}
	log.Info("Gitea is now ready")

	username := req.Parameters["username"]
	password := req.Parameters["password"]
	email := req.Parameters["email"]
	gitName := req.Parameters["git_name"]
	repoName := req.Parameters["repo_name"]
	sshPort := req.Parameters["ssh_port"]
	if sshPort == "" {
		sshPort = "22"
	}

	// Ensure .ssh directory exists
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error creating .ssh directory: %v", err),
		}, nil
	}

	// Check if ssh-keygen is installed
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "ssh-keygen command not found. Please ensure it's installed and in your PATH.",
		}, nil
	}

	// Generate a unique identifier
	uniqueID, err := generateUniqueID()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error generating unique ID: %v", err),
		}, nil
	}

	// Create a unique filename for the SSH key
	sshKeyName := fmt.Sprintf("id_ed25519_gitea_%s_%s", username, uniqueID)
	sshKeyPath := filepath.Join(sshDir, sshKeyName)

	// Generate SSH key
	log.Info("Generating SSH key...")
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", sshKeyPath, "-N", "")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to generate SSH key", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error generating SSH key: %v\nOutput: %s", err, string(output)),
		}, nil
	}
	log.Info("SSH key generated successfully")

	// Update SSH config with the new key name
	log.Info("Updating SSH config...")
	configPath := filepath.Join(sshDir, "config")
	configContent := fmt.Sprintf(`
Host gitea-local-%s
    HostName localhost
    Port %s
    User git
    IdentityFile %s
`, uniqueID, sshPort, sshKeyPath)
	if err := appendToFile(configPath, configContent); err != nil {
		log.Error("Failed to update SSH config", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error updating SSH config: %v", err),
		}, nil
	}
	log.Info("SSH config updated successfully")

	// Upload SSH key to Gitea
	log.Info("Uploading SSH key to Gitea...")
	sshKey, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		log.Error("Failed to read SSH public key", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error reading SSH key: %v", err)}, nil
	}
	keyTitle := "Automated SSH Key"
	if err := uploadSSHKey(username, password, string(sshKey), keyTitle); err != nil {
		log.Error("Failed to upload SSH key to Gitea", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error uploading SSH key: %v", err)}, nil
	}
	log.Info("SSH key uploaded successfully")

	// Clone the repository
	log.Info("Cloning repository...")
	if err := cloneRepo(repoName, username); err != nil {
		log.Error("Failed to clone repository", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error cloning repository: %v", err)}, nil
	}
	log.Info("Repository cloned successfully")

	// Set local Git configuration
	log.Info("Setting local Git configuration...")
	if err := setGitConfig(repoName, gitName, email); err != nil {
		log.Error("Failed to set Git config", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error setting Git config: %v", err)}, nil
	}
	log.Info("Local Git configuration set successfully")

	// Update remote URL
	log.Info("Updating remote URL...")
	if err := updateRemoteURL(repoName, username, uniqueID); err != nil {
		log.Error("Failed to update remote URL", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error updating remote URL: %v", err)}, nil
	}
	log.Info("Remote URL updated successfully")

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

func updateRemoteURL(repoName, username, uniqueID string) error {
	cmd := exec.Command("git", "-C", repoName, "remote", "set-url", "origin", fmt.Sprintf("git@gitea-local-%s:%s/%s.git", uniqueID, username, repoName))
	return cmd.Run()
}

func generateUniqueID() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	combined := append([]byte(fmt.Sprintf("%d", timestamp)), randomBytes...)
	return hex.EncodeToString(combined), nil
}

func waitForGitea() error {
	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 60; i++ { // Try for 60 seconds
		resp, err := client.Get("http://localhost:3000/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil // Gitea is up
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("Gitea did not start within the expected time")
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

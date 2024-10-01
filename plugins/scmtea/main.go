package main

import (
	"bytes"
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
	"github.com/pelletier/go-toml"
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

type DefaultValues struct {
	Gitea struct {
		LastUpdated time.Time `toml:"last_updated"`
		Username    string    `toml:"username"`
		Password    string    `toml:"password"`
		Email       string    `toml:"email"`
		GitName     string    `toml:"git_name"`
		RepoName    string    `toml:"repo_name"`
		SSHPort     string    `toml:"ssh_port"`
	} `toml:"gitea"`
}

type ScmteaPlugin struct {
	logger *logger.RateLimitedLogger
}

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
		// This is just a submenu entry point, so we don't need to do anything here
		return &pb.CommandResponse{
			Success: true,
			Result:  "Select an option from the Docker Compose submenu",
		}, nil
	case "set_compose_file_default":
		return setComposeFile("Use default", "")
	case "set_compose_file_custom":
		customPath, ok := req.Parameters["custom_path"]
		if !ok || customPath == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required for set_compose_file_custom command",
			}, nil
		}
		return setComposeFile("Enter custom path", customPath)
	case "setup":
		return setupGitea(req)
	case "start":
		return runDockerCompose("up", "-d")
	case "stop":
		return runDockerCompose("down")
	case "restart":
		return runDockerCompose("restart")
	case "print_summary":
		summary, err := printGiteaSummary(p.logger)
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to print Gitea summary: %v", err),
			}, nil
		}
		return &pb.CommandResponse{
			Success: true,
			Result:  summary,
		}, nil
	case "git_config_summary":
		return gitConfigSummary()
	case "delete_containers_images":
		return deleteContainersAndImages()
	case "delete_volumes":
		return deleteVolumes()
	case "go_back":
		return &pb.CommandResponse{
			Success: true,
			Result:  "Returned to previous menu",
		}, nil
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
			SubMenu: []gsplug.MenuOption{
				{Label: "Use Default Docker Compose File", Command: "set_compose_file_default"},
				{
					Label:   "Enter Custom Docker Compose Path",
					Command: "set_compose_file_custom",
					Parameters: []gsplug.ParameterInfo{
						{Name: "custom_path", Description: "Path to custom Docker Compose file", Required: true},
					},
				},
			},
		},
		{
			Label:   "Setup Gitea",
			Command: "setup",
			Parameters: []gsplug.ParameterInfo{
				{Name: "username", Description: "Gitea username", Required: true},
				{Name: "password", Description: "Gitea password (warning: will be unmasked/displayed in terminal for now)", Required: true},
				{Name: "email", Description: "Gitea email", Required: true},
				{Name: "git_name", Description: "Name for Git commits", Required: true},
				{Name: "repo_name", Description: "Repository name", Required: true},
				{Name: "ssh_port", Description: "SSH port for Gitea (default is 22)", Required: false},
			},
		},
		{Label: "Start Gitea", Command: "start"},
		{Label: "Stop Gitea", Command: "stop"},
		{Label: "Restart Gitea", Command: "restart"},
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

	log.Info("Setting compose file", "dataDir", dataDir, "destPath", destPath)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Error("Failed to create plugin data directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create plugin data directory: %v", err),
		}, nil
	}

	switch option {
	case "Use default":
		log.Info("Using default compose file")
		defaultCompose, err := defaultComposeFile.ReadFile(defaultComposeFileName)
		if err != nil {
			log.Error("Failed to read default docker-compose.yaml", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read default docker-compose.yaml: %v", err),
			}, nil
		}
		if err = ioutil.WriteFile(destPath, defaultCompose, 0644); err != nil {
			log.Error("Failed to write default docker-compose.yaml", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to write default docker-compose.yaml: %v", err),
			}, nil
		}
		log.Info("Default compose file written successfully", "path", destPath)
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

	// Log the content of the docker-compose file
	composeContent, err := ioutil.ReadFile(composePath)
	if err != nil {
		log.Error("Error reading docker-compose file", "error", err)
	} else {
		log.Info("Docker-compose file content", "content", string(composeContent))
	}

	// Try docker-compose command
	log.Info("Attempting to run docker-compose command")
	cmdArgs := append([]string{"-f", composePath}, args...)
	cmd := exec.Command("docker-compose", cmdArgs...)
	log.Info("Full docker-compose command", "command", cmd.String())
	output, err := cmd.CombinedOutput()
	log.Info("docker-compose command output", "output", string(output))

	if err != nil {
		log.Error("docker-compose command failed, attempting docker compose", "error", err)
		// If docker-compose fails, try docker compose
		cmd = exec.Command("docker", append([]string{"compose", "-f", composePath}, args...)...)
		log.Info("Full docker compose command", "command", cmd.String())
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

func printGiteaSummary(logger *logger.RateLimitedLogger) (string, error) {
	logger.Debug("Starting printGiteaSummary function")

	// Helper function to run Docker commands and log output
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

	// Check Docker daemon
	_, err := runDockerCommand("Check Docker", "version")
	if err != nil {
		return "", fmt.Errorf("Docker daemon is not accessible: %v", err)
	}

	// List all running containers
	allContainers, err := runDockerCommand("List containers", "ps", "--format", "{{.Names}}")
	if err != nil {
		return "", fmt.Errorf("Failed to list containers: %v", err)
	}
	logger.Debug("All running containers", "containers", allContainers)

	// Find Gitea and DB containers
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

	logger.Debug("Found Gitea containers", "gitea", giteaContainer, "db", dbContainer)

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

	dbPort := "5432" // Default Postgres port
	dbPortMapping, err := runDockerCommand("Get DB port mapping", "port", dbContainer)
	if err != nil {
		logger.Warn("Failed to get DB port mapping, using internal port", "error", err)
	} else if dbPortMapping != "" {
		dbPort = strings.Split(dbPortMapping, ":")[0]
	}

	dbIp, err := runDockerCommand("Get DB IP", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", dbContainer)
	if err != nil {
		logger.Warn("Failed to get DB IP", "error", err)
		dbIp = "N/A"
	}

	summary := fmt.Sprintf(`Gitea Summary:
Gitea Container:
  Name: %s
  Web UI: http://localhost:%s
  SSH: ssh://localhost:%s
  Internal IP: %s

Database Container:
  Name: %s
  Port: %s (internal)
  Internal IP: %s`,
		giteaContainer,
		strings.TrimPrefix(giteaPort, "0.0.0.0:"),
		strings.TrimPrefix(giteaSshPort, "0.0.0.0:"),
		giteaIp,
		dbContainer,
		dbPort,
		dbIp)

	logger.Debug("Generated summary", "summary", summary)
	return summary, nil
}

func deleteContainersAndImages() (*pb.CommandResponse, error) {
	log.Info("Starting deleteContainersAndImages")

	// Check Docker daemon status
	if err := checkDockerStatus(); err != nil {
		log.Error("Docker daemon is not running or accessible", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Docker daemon is not running or accessible: %v", err)}, nil
	}

	// Stop and remove containers
	log.Info("Stopping and removing containers with docker-compose down")
	downOutput, err := runDockerCompose("down")
	if err != nil {
		log.Error("Error stopping containers with docker-compose down", "error", err, "output", downOutput)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error stopping containers with docker-compose down: %v\nOutput: %s", err, downOutput)}, nil
	}
	log.Info("docker-compose down completed successfully")

	// Check for running containers
	runningContainers, err := getRunningContainers()
	if err != nil {
		log.Error("Error checking for running containers", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error checking for running containers: %v", err)}, nil
	}

	if len(runningContainers) > 0 {
		log.Warn("Containers are still running after docker-compose down, attempting to force stop", "containers", runningContainers)
		for _, container := range runningContainers {
			stopOutput, err := exec.Command("docker", "stop", container).CombinedOutput()
			if err != nil {
				log.Error("Error force stopping container", "container", container, "error", err, "output", string(stopOutput))
				return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error force stopping container %s: %v\nOutput: %s", container, err, stopOutput)}, nil
			}
			log.Info("Container force stopped successfully", "container", container)
		}
	}

	// Remove Gitea images
	if err := removeImages("gitea/gitea"); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	// Remove Postgres images
	if err := removeImages("postgres:13"); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	log.Info("deleteContainersAndImages completed successfully")
	return &pb.CommandResponse{
		Success: true,
		Result:  "Containers and images have been deleted.",
	}, nil
}

func checkDockerStatus() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running or accessible: %v", err)
	}
	return nil
}

func getRunningContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "-q", "--filter", "name=gitea")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Error listing running containers: %v\nOutput: %s", err, output)
	}
	containers := strings.Fields(string(output))
	return containers, nil
}

func removeImages(imageName string) error {
	log.Info("Attempting to remove images", "image", imageName)
	images, err := exec.Command("docker", "images", imageName, "-q").Output()
	if err != nil {
		log.Error("Error listing images", "image", imageName, "error", err)
		return fmt.Errorf("Error listing %s images: %v", imageName, err)
	}

	log.Info("Images found", "image", imageName, "count", len(strings.Fields(string(images))))
	if len(images) > 0 {
		cmd := exec.Command("docker", "rmi", "-f", strings.TrimSpace(string(images)))
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Error("Error removing images", "image", imageName, "error", err, "output", string(output))
			return fmt.Errorf("Error removing %s images: %v\nOutput: %s", imageName, err, output)
		}
		log.Info("Images removed successfully", "image", imageName)
	} else {
		log.Info("No images found to remove", "image", imageName)
	}
	return nil
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
	log.Info("Gitea containers started successfully", "output", startResponse.Result)

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

	// Get the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get user home directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get user home directory: %v", err),
		}, nil
	}

	// Construct the path to setup_gitea.js
	setupScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "setup_gitea.js")

	// Check if the file exists
	if _, err := os.Stat(setupScriptPath); os.IsNotExist(err) {
		log.Error("setup_gitea.js not found", "path", setupScriptPath)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("setup_gitea.js not found at %s", setupScriptPath),
		}, nil
	}

	// Run the setup_gitea.js script
	log.Info("Running Gitea setup script...")
	cmd := exec.Command("node", setupScriptPath,
		req.Parameters["username"],
		req.Parameters["email"],
		req.Parameters["password"])

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Gitea setup script failed", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Gitea setup script failed: %v\nOutput: %s", err, output),
		}, nil
	}

	log.Info("Gitea setup script completed", "output", string(output))

	// Parse the JSON output
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	// Find the start of the JSON output
	jsonStart := bytes.LastIndex(output, []byte("{"))
	if jsonStart == -1 {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Failed to find JSON output in script response",
		}, nil
	}
	jsonOutput := output[jsonStart:]

	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		log.Error("Failed to parse setup script output", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse setup script output: %v\nRaw output: %s", err, output),
		}, nil
	}

	if !result.Success {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: result.Message,
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  result.Message,
	}, nil
}

func generateAndUploadSSHKey(params map[string]string) (string, error) {
	// Generate SSH key
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("error creating .ssh directory: %v", err)
	}

	uniqueID, err := generateUniqueID()
	if err != nil {
		return "", fmt.Errorf("error generating unique ID: %v", err)
	}

	sshKeyName := fmt.Sprintf("id_ed25519_gitea_%s_%s", params["username"], uniqueID)
	sshKeyPath := filepath.Join(sshDir, sshKeyName)

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", params["email"], "-f", sshKeyPath, "-N", "")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("error generating SSH key: %v\nOutput: %s", err, output)
	}

	// Read public key
	pubKeyBytes, err := ioutil.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("error reading public key: %v", err)
	}

	// Upload SSH key
	client := &http.Client{}
	uploadURL := "http://localhost:3000/api/v1/user/keys"
	data := map[string]string{
		"title": "Gitea SSH Key",
		"key":   string(pubKeyBytes),
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequest("POST", uploadURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(params["username"], params["password"])

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error uploading SSH key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("SSH key upload failed with status %s: %s", resp.Status, body)
	}

	return sshKeyPath, nil
}

func waitForGitea() error {
	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 120; i++ { // Try for 2 minutes
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

func automateGiteaSetup(username, password, email, gitName, repoName string) error {
	// Get the path to the plugin directory
	pluginDir, err := getPluginDir()
	if err != nil {
		return fmt.Errorf("failed to get plugin directory: %w", err)
	}

	scriptPath := filepath.Join(pluginDir, "setup_gitea.js")

	cmd := exec.Command("node", scriptPath,
		username,
		password,
		email,
		gitName,
		repoName)

	log.Info("Running Gitea setup script", "path", scriptPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run Gitea setup script: %w\nOutput: %s", err, string(output))
	}
	log.Info("Gitea setup script output", "output", string(output))
	return nil
}

func getPluginDir() (string, error) {
	// This assumes the plugin binary is in the plugin directory
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return filepath.Dir(ex), nil
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
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to upload SSH key, status: %s, body: %s", resp.Status, string(body))
	}
	return nil
}

func createAndCloneRepo(repoName, username, password string) error {
	// Create repository
	createURL := fmt.Sprintf("http://localhost:3000/api/v1/user/repos")
	payload := fmt.Sprintf(`{"name":"%s","auto_init":true}`, repoName)
	req, err := http.NewRequest("POST", createURL, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create repository, status: %s, body: %s", resp.Status, string(body))
	}

	// Clone repository
	cloneURL := fmt.Sprintf("http://localhost:3000/%s/%s.git", username, repoName)
	cmd := exec.Command("git", "clone", cloneURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %v\nOutput: %s", err, string(output))
	}
	return nil
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

func main() {
	logger, err := logger.NewRateLimitedLogger("scmtea")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Scmtea plugin starting")

	plugin := &ScmteaPlugin{
		logger: logger,
	}

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

		// Flush stdout to ensure the message is sent immediately
		os.Stdout.Sync()
	}
}

func createSampleRepo(username, repoName string) error {
	// Create a new repository on Gitea
	createURL := fmt.Sprintf("http://localhost:3000/api/v1/user/repos")
	payload := fmt.Sprintf(`{"name":"%s","auto_init":true}`, repoName)
	req, err := http.NewRequest("POST", createURL, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(username, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create repository, status: %s, body: %s", resp.Status, string(body))
	}

	// Clone the newly created repository
	cloneURL := fmt.Sprintf("http://localhost:3000/%s/%s.git", username, repoName)
	cmd := exec.Command("git", "clone", cloneURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(output))
	}

	log.Info("Sample repository created and cloned successfully")
	return nil
}

func cloneRepo(repoName, username string) error {
	cmd := exec.Command("git", "clone", fmt.Sprintf("http://localhost:3000/%s/%s.git", username, repoName))
	return cmd.Run()
}

func readDefaultValues() (DefaultValues, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultValues{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultsPath := filepath.Join(homeDir, ".ssot", "gitspace", "data", "scmtea", "defaults.toml")

	var defaults DefaultValues
	tree, err := toml.LoadFile(defaultsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, return empty defaults
			return DefaultValues{}, nil
		}
		return DefaultValues{}, fmt.Errorf("failed to read defaults file: %w", err)
	}

	err = tree.Unmarshal(&defaults)
	if err != nil {
		return DefaultValues{}, fmt.Errorf("failed to unmarshal defaults: %w", err)
	}

	return defaults, nil
}

func updateDefaultValues(values DefaultValues) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultsPath := filepath.Join(homeDir, ".ssot", "gitspace", "data", "scmtea", "defaults.toml")

	values.Gitea.LastUpdated = time.Now()

	f, err := os.Create(defaultsPath)
	if err != nil {
		return fmt.Errorf("failed to create defaults file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(values); err != nil {
		return fmt.Errorf("failed to encode defaults: %w", err)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

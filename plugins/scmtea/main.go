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
	case "generate_ssh_key":
		return generateAndUploadSSHKey(req)
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
				{Name: "password", Description: "Gitea password", Required: true},
				{Name: "email", Description: "Gitea email", Required: true},
			},
		},
		{Label: "Start Gitea", Command: "start"},
		{
			Label:   "Generate and Upload SSH Key",
			Command: "generate_ssh_key",
			Parameters: []gsplug.ParameterInfo{
				{Name: "username", Description: "Gitea username", Required: true},
				{Name: "password", Description: "Gitea password", Required: true},
				{Name: "email", Description: "Gitea email", Required: true},
			},
		},
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

	composeContent, err := ioutil.ReadFile(composePath)
	if err != nil {
		log.Error("Error reading docker-compose file", "error", err)
	} else {
		log.Info("Docker-compose file content", "content", string(composeContent))
	}

	log.Info("Attempting to run docker-compose command")
	cmdArgs := append([]string{"-f", composePath}, args...)
	cmd := exec.Command("docker-compose", cmdArgs...)
	log.Info("Full docker-compose command", "command", cmd.String())
	output, err := cmd.CombinedOutput()
	log.Info("docker-compose command output", "output", string(output))

	if err != nil {
		log.Error("docker-compose command failed, attempting docker compose", "error", err)
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

	if err := checkDockerStatus(); err != nil {
		log.Error("Docker daemon is not running or accessible", "error", err)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Docker daemon is not running or accessible: %v", err)}, nil
	}

	log.Info("Stopping and removing containers with docker-compose down")
	downOutput, err := runDockerCompose("down")
	if err != nil {
		log.Error("Error stopping containers with docker-compose down", "error", err, "output", downOutput)
		return &pb.CommandResponse{Success: false, ErrorMessage: fmt.Sprintf("Error stopping containers with docker-compose down: %v\nOutput: %s", err, downOutput)}, nil
	}
	log.Info("docker-compose down completed successfully")

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

	if err := removeImages("gitea/gitea"); err != nil {
		return &pb.CommandResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

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
	log.Info("Starting Gitea containers...")
	startResponse, err := runDockerCompose("up", "-d")
	if err != nil {
		log.Error("Failed to start Gitea containers", "error", err)
		return startResponse, err
	}
	log.Info("Gitea containers started successfully", "output", startResponse.Result)

	log.Info("Waiting for Gitea to be ready...")
	if err := waitForGitea(); err != nil {
		log.Error("Gitea failed to start within the expected time", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error waiting for Gitea to start: %v", err),
		}, nil
	}
	log.Info("Gitea is now ready")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get user home directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get user home directory: %v", err),
		}, nil
	}

	setupScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "setup_gitea.js")

	if _, err := os.Stat(setupScriptPath); os.IsNotExist(err) {
		log.Error("setup_gitea.js not found", "path", setupScriptPath)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("setup_gitea.js not found at %s", setupScriptPath),
		}, nil
	}

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

	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	var jsonLines []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			jsonLines = append(jsonLines, line)
		}
	}

	if len(jsonLines) == 0 {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("No JSON output found in script output: %s", output),
		}, nil
	}

	if err := json.Unmarshal([]byte(jsonLines[len(jsonLines)-1]), &result); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error parsing script output: %v\nOutput: %s", err, output),
		}, nil
	}

	if !result.Success {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("SSH key upload failed: %s\nFull log:\n%s", result.Message, strings.Join(jsonLines, "\n")),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  result.Message,
	}, nil
}

func generateAndUploadSSHKey(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	username := req.Parameters["username"]
	password := req.Parameters["password"]
	email := req.Parameters["email"]

	if username == "" || password == "" || email == "" {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Missing required parameters: username, password, and email are required",
		}, nil
	}

	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		log.Error("Failed to create .ssh directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error creating .ssh directory: %v", err),
		}, nil
	}

	uniqueID, err := generateUniqueID()
	if err != nil {
		log.Error("Failed to generate unique ID", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error generating unique ID: %v", err),
		}, nil
	}

	sshKeyName := fmt.Sprintf("id_ed25519_gitea_%s_%s", username, uniqueID)
	sshKeyPath := filepath.Join(sshDir, sshKeyName)

	log.Info("Generating SSH key", "path", sshKeyPath)
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", sshKeyPath, "-N", "")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to generate SSH key", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error generating SSH key: %v\nOutput: %s", err, output),
		}, nil
	}

	pubKeyBytes, err := ioutil.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		log.Error("Failed to read public key", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error reading public key: %v", err),
		}, nil
	}
	pubKey := string(pubKeyBytes)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get user home directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get user home directory: %v", err),
		}, nil
	}

	uploadScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "ssh-key", "index.js")

	if _, err := os.Stat(uploadScriptPath); os.IsNotExist(err) {
		log.Error("upload_ssh_key.js not found", "path", uploadScriptPath)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("upload_ssh_key.js not found at %s", uploadScriptPath),
		}, nil
	}

	log.Info("Running SSH key upload script...")
	cmd = exec.Command("bun", "run", uploadScriptPath, username, password, pubKey)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Error("SSH key upload script failed", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("SSH key upload script failed: %v\nOutput: %s", err, output),
		}, nil
	}

	log.Info("SSH key upload script completed", "output", string(output))

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	var jsonLines []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			jsonLines = append(jsonLines, line)
		}
	}

	if len(jsonLines) == 0 {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("No JSON output found in script output: %s", output),
		}, nil
	}

	if err := json.Unmarshal([]byte(jsonLines[len(jsonLines)-1]), &result); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error parsing script output: %v\nOutput: %s", err, output),
		}, nil
	}

	if !result.Success {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("SSH key upload failed: %s\nFull log:\n%s", result.Message, strings.Join(jsonLines, "\n")),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("SSH key generated and uploaded successfully. Private key path: %s", sshKeyPath),
	}, nil
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

func generateUniqueID() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	combined := append([]byte(fmt.Sprintf("%d", timestamp)), randomBytes...)
	return hex.EncodeToString(combined), nil
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

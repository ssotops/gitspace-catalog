package main

import (
	"bufio"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
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

// Plugin structures
type ScmteaPlugin struct {
	logger *logger.RateLimitedLogger
}

type DefaultValues struct {
	Gitea struct {
		LastUpdated time.Time `toml:"last_updated"`
		Username    string    `toml:"username"`
		Password    string    `toml:"password"`
		Email       string    `toml:"email"`
	} `toml:"gitea"`
}

// Progress message structure
type ProgressMessage struct {
	Status  string `json:"status"`
	Step    string `json:"step"`
	Message string `json:"message"`
}

// Plugin interface methods
func (p *ScmteaPlugin) GetPluginInfo(req *pb.PluginInfoRequest) (*pb.PluginInfo, error) {
	log.Info("GetPluginInfo called")
	return &pb.PluginInfo{
		Name:    "Scmtea Plugin",
		Version: "1.0.0",
	}, nil
}

func (p *ScmteaPlugin) GetMenu(req *pb.MenuRequest) (*pb.MenuResponse, error) {
	menuOptions := []gsplug.MenuOption{
		{
			Label:   "Installation",
			Command: "installation_menu",
			SubMenu: []gsplug.MenuOption{
				{
					Label:   "Set Docker Compose File",
					Command: "set_compose_file",
					SubMenu: []gsplug.MenuOption{
						{
							Label:   "Use Default Docker Compose File",
							Command: "set_compose_file_default",
						},
						{
							Label:   "Enter Custom Docker Compose Path",
							Command: "set_compose_file_custom",
							Parameters: []gsplug.ParameterInfo{
								{
									Name:        "custom_path",
									Description: "Path to custom Docker Compose file",
									Required:    true,
								},
							},
						},
					},
				},
				{
					Label:   "Setup Gitea",
					Command: "setup",
					Parameters: []gsplug.ParameterInfo{
						{
							Name:        "username",
							Description: "Gitea username",
							Required:    true,
						},
						{
							Name:        "password",
							Description: "Gitea password",
							Required:    true,
						},
						{
							Name:        "email",
							Description: "Gitea email",
							Required:    true,
						},
					},
				},
				{
					Label:   "Generate and Upload SSH Key",
					Command: "generate_ssh_key",
					Parameters: []gsplug.ParameterInfo{
						{
							Name:        "username",
							Description: "Gitea username",
							Required:    true,
						},
						{
							Name:        "password",
							Description: "Gitea password",
							Required:    true,
						},
						{
							Name:        "email",
							Description: "Gitea email",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Label:   "Lifecycle Management",
			Command: "lifecycle_menu",
			SubMenu: []gsplug.MenuOption{
				{
					Label:   "Start Gitea",
					Command: "start",
				},
				{
					Label:   "Stop Gitea",
					Command: "stop",
				},
				{
					Label:   "Restart Gitea",
					Command: "restart",
				},
				{
					Label:   "Delete Gitea Containers and Images",
					Command: "delete_containers_images",
				},
				{
					Label:   "Delete Volumes",
					Command: "delete_volumes",
				},
			},
		},
		{
			Label:   "Backup Management",
			Command: "backup_menu",
			SubMenu: []gsplug.MenuOption{
				{
					Label:   "Configure Backup Storage",
					Command: "configure_backup",
					Parameters: []gsplug.ParameterInfo{
						{
							Name:        "s3_bucket",
							Description: "S3 bucket name",
							Required:    true,
						},
						{
							Name:        "s3_path",
							Description: "Path within bucket",
							Required:    false,
						},
						{
							Name:        "access_key",
							Description: "S3 access key",
							Required:    true,
						},
						{
							Name:        "secret_key",
							Description: "S3 secret key",
							Required:    true,
						},
						{
							Name:        "endpoint",
							Description: "S3 endpoint (e.g., nyc3.digitaloceanspaces.com)",
							Required:    true,
						},
					},
				},
				{
					Label:   "Set Backup Schedule",
					Command: "set_backup_schedule",
					Parameters: []gsplug.ParameterInfo{
						{
							Name:        "schedule",
							Description: "Cron expression (e.g., '0 2 * * *' for daily at 2 AM)",
							Required:    true,
						},
					},
				},
				{
					Label:   "Create Backup Now",
					Command: "create_backup",
				},
				{
					Label:   "Restore from Backup",
					Command: "restore_backup",
					Parameters: []gsplug.ParameterInfo{
						{
							Name:        "backup_file",
							Description: "Path to backup file",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Label:   "Information",
			Command: "info_menu",
			SubMenu: []gsplug.MenuOption{
				{
					Label:   "Print Gitea Summary",
					Command: "print_summary",
				},
				{
					Label:   "Print Git Config Summary",
					Command: "git_config_summary",
				},
			},
		},
	}

	menuBytes, err := json.Marshal(menuOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal menu: %w", err)
	}

	return &pb.MenuResponse{
		MenuData: menuBytes,
	}, nil
}

func (p *ScmteaPlugin) ExecuteCommand(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	p.logger.Info("Executing command", "command", req.Command, "parameters", req.Parameters)

	switch req.Command {
	case "set_compose_file":
		return &pb.CommandResponse{
			Success: true,
			Result:  "Select an option from the Docker Compose submenu",
		}, nil

	case "set_compose_file_default":
		p.logger.Info("Setting default compose file")
		return setComposeFile("Use default", "")

	case "set_compose_file_custom":
		customPath, ok := req.Parameters["custom_path"]
		if !ok || customPath == "" {
			p.logger.Error("Missing custom path parameter")
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required for set_compose_file_custom command",
			}, nil
		}
		p.logger.Info("Setting custom compose file", "path", customPath)
		return setComposeFile("Enter custom path", customPath)

	case "setup":
		p.logger.Info("Starting Gitea setup process")

		// Pre-setup environment checks
		// 1. Check if Node.js is available
		if _, err := exec.LookPath("node"); err != nil {
			p.logger.Error("Node.js not found in PATH", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Node.js is required but not found in PATH. Please install Node.js and try again.",
			}, nil
		}
		p.logger.Info("Node.js found in PATH")

		// 2. Check if Docker is running
		dockerCmd := exec.Command("docker", "info")
		if err := dockerCmd.Run(); err != nil {
			p.logger.Error("Docker is not running", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Docker is not running. Please start Docker and try again.",
			}, nil
		}
		p.logger.Info("Docker is running")

		// 3. Check if port 3000 is available
		conn, err := net.DialTimeout("tcp", "localhost:3000", time.Second)
		if err == nil {
			conn.Close()
			p.logger.Error("Port 3000 is already in use")
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Port 3000 is already in use. Please ensure no other service is using this port.",
			}, nil
		}
		p.logger.Info("Port 3000 is available")

		// 4. Get setup script path and verify it exists
		homeDir, err := os.UserHomeDir()
		if err != nil {
			p.logger.Error("Failed to get home directory", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to get home directory: %v", err),
			}, nil
		}

		setupScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "setup_gitea.js")
		if _, err := os.Stat(setupScriptPath); os.IsNotExist(err) {
			p.logger.Error("Setup script not found", "path", setupScriptPath)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("setup_gitea.js not found at %s. Please ensure the plugin is properly installed.", setupScriptPath),
			}, nil
		}
		p.logger.Info("Setup script found", "path", setupScriptPath)

		// Validate required parameters
		for _, param := range []string{"username", "password", "email"} {
			if _, ok := req.Parameters[param]; !ok {
				p.logger.Error("Missing required parameter", "param", param)
				return &pb.CommandResponse{
					Success:      false,
					ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
				}, nil
			}
		}
		p.logger.Info("All required parameters present")

		// Start Docker containers
		p.logger.Info("Starting Docker containers")
		startResponse, err := runDockerCompose("up", "-d")
		if err != nil {
			p.logger.Error("Failed to start Docker containers", "error", err)
			return startResponse, nil
		}
		p.logger.Info("Docker containers started successfully")

		// Define progress callback for health check
		sendProgress := func(status, step, message string) {
			p.logger.Info("Setup progress",
				"status", status,
				"step", step,
				"message", message)
		}

		// Wait for Gitea to be available
		p.logger.Info("Waiting for Gitea to be ready")
		if err := waitForGiteaWithProgress(sendProgress); err != nil {
			p.logger.Error("Failed waiting for Gitea", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Error waiting for Gitea to start: %v", err),
			}, nil
		}
		p.logger.Info("Gitea is now ready")

		// Run setup script
		p.logger.Info("Starting Gitea setup script",
			"username", req.Parameters["username"],
			"email", req.Parameters["email"])

		cmd := exec.Command("node", setupScriptPath,
			req.Parameters["username"],
			req.Parameters["email"],
			req.Parameters["password"])

		// Set up output pipes
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			p.logger.Error("Failed to create stdout pipe", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to create stdout pipe: %v", err),
			}, nil
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			p.logger.Error("Failed to create stderr pipe", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to create stderr pipe: %v", err),
			}, nil
		}

		// Start the command
		p.logger.Info("Executing setup script")
		if err := cmd.Start(); err != nil {
			p.logger.Error("Failed to start setup script", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to start setup script: %v", err),
			}, nil
		}

		// Process stdout in real-time
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()

				// Try to parse as JSON progress message
				var progressMsg struct {
					Type    string `json:"type"`
					Payload struct {
						Status  string `json:"status"`
						Step    string `json:"step"`
						Details string `json:"details"`
					} `json:"payload"`
				}

				if err := json.Unmarshal([]byte(line), &progressMsg); err != nil {
					// Not JSON, log as plain text
					p.logger.Info("Setup output", "message", line)
				} else {
					// Log structured progress message
					p.logger.Info("Setup progress",
						"type", progressMsg.Type,
						"status", progressMsg.Payload.Status,
						"step", progressMsg.Payload.Step,
						"details", progressMsg.Payload.Details)
				}
			}

			if err := scanner.Err(); err != nil {
				p.logger.Error("Error reading setup script output", "error", err)
			}
		}()

		// Process stderr in real-time
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				p.logger.Error("Setup script error", "message", scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				p.logger.Error("Error reading setup script stderr", "error", err)
			}
		}()

		// Wait for completion
		p.logger.Info("Waiting for setup script to complete")
		if err := cmd.Wait(); err != nil {
			p.logger.Error("Setup script failed", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Setup script failed: %v", err),
			}, nil
		}

		p.logger.Info("Setup completed successfully")
		return &pb.CommandResponse{
			Success: true,
			Result:  "Gitea setup completed successfully",
		}, nil

	case "generate_ssh_key":
		return generateAndUploadSSHKey(req)

	case "start":
		return runDockerCompose("up", "-d")

	case "stop":
		return runDockerCompose("down")

	case "restart":
		return runDockerCompose("restart")

	case "delete_containers_images":
		return deleteContainersAndImages()

	case "delete_volumes":
		return deleteVolumes()

	// Backup commands
	case "configure_backup":
		if err := validateBackupConfig(req.Parameters); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Invalid backup configuration: %v", err),
			}, nil
		}
		return p.handleBackupCommands(req)

	case "create_backup":
		return p.handleBackupCommands(req)

	case "set_backup_schedule":
		schedule, ok := req.Parameters["schedule"]
		if !ok || schedule == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Backup schedule (cron expression) is required",
			}, nil
		}
		return p.handleBackupCommands(req)

	case "restore_backup":
		backupFile, ok := req.Parameters["backup_file"]
		if !ok || backupFile == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Backup file path is required",
			}, nil
		}
		return p.handleBackupCommands(req)

	case "view_backup_summary":
		return p.handleBackupCommands(req)

	default:
		p.logger.Error("Unknown command", "command", req.Command)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Unknown command: %s", req.Command),
		}, nil
	}
}

func setupGitea(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	// Validate required parameters
	for _, param := range []string{"username", "password", "email"} {
		if _, ok := req.Parameters[param]; !ok {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
			}, nil
		}
	}

	// Start Docker containers
	startResponse, err := runDockerCompose("up", "-d")
	if err != nil {
		return startResponse, err
	}

	// Wait for Gitea to be ready
	if err := waitForGiteaWithProgress(func(status, step, message string) {
		log.Info(fmt.Sprintf("[%s] %s: %s", status, step, message))
	}); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error waiting for Gitea to start: %v", err),
		}, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get home directory: %v", err),
		}, nil
	}

	setupScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "setup_gitea.js")
	if _, err := os.Stat(setupScriptPath); os.IsNotExist(err) {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("setup_gitea.js not found at %s", setupScriptPath),
		}, nil
	}

	// Run setup script with real-time output processing
	cmd := exec.Command("node", setupScriptPath,
		req.Parameters["username"],
		req.Parameters["email"],
		req.Parameters["password"])

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create stdout pipe: %v", err),
		}, nil
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create stderr pipe: %v", err),
		}, nil
	}

	if err := cmd.Start(); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to start setup script: %v", err),
		}, nil
	}

	// Process script output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var msg struct {
				Type    string `json:"type"`
				Payload struct {
					Status  string `json:"status"`
					Step    string `json:"step"`
					Details string `json:"details"`
				} `json:"payload"`
			}

			if err := json.Unmarshal([]byte(scanner.Text()), &msg); err != nil {
				// Log raw output if it's not in our expected format
				log.Info("Setup output", "message", scanner.Text())
				continue
			}

			// Log progress update
			switch msg.Type {
			case "progress":
				log.Info(fmt.Sprintf("[%s] %s: %s",
					msg.Payload.Status,
					msg.Payload.Step,
					msg.Payload.Details))
			case "result":
				// Final result will be handled after cmd.Wait()
				continue
			}
		}
	}()

	// Process stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Error("Setup error", "message", scanner.Text())
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Setup script failed: %v", err),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Gitea setup completed successfully",
	}, nil
}

func waitForGiteaWithProgress(progressFn func(status, step, message string)) error {
	client := &http.Client{Timeout: 1 * time.Second}
	maxAttempts := 120 // 2 minutes

	progressFn("info", "Startup", fmt.Sprintf("Beginning health check for Gitea. Will try for %d seconds.", maxAttempts))

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		progressFn("info", "Health Check", fmt.Sprintf("Attempt %d of %d: Checking http://localhost:3000", attempt, maxAttempts))

		resp, err := client.Get("http://localhost:3000/")
		if err != nil {
			progressFn("info", "Health Check", fmt.Sprintf("Connection failed (attempt %d): %v", attempt, err))
			time.Sleep(1 * time.Second)
			continue
		}

		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			progressFn("success", "Health Check", "Gitea server responded with 200 OK")
			return nil
		}

		progressFn("info", "Health Check", fmt.Sprintf("Got HTTP %d (attempt %d), waiting...", resp.StatusCode, attempt))
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("Gitea did not become available within %d seconds", maxAttempts)
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

	log.Info("Running SSH key upload script...")
	cmd = exec.Command("bun", "run", uploadScriptPath, username, password, pubKey)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("SSH key upload script failed", "error", err, "output", string(cmdOutput))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("SSH key upload script failed: %v\nOutput: %s", err, cmdOutput),
		}, nil
	}

	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(cmdOutput, &result); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse script output: %v\nOutput: %s", err, cmdOutput),
		}, nil
	}

	if !result.Success {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("SSH key upload failed: %s", result.Message),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("SSH key generated and uploaded successfully. Private key path: %s", sshKeyPath),
	}, nil
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

func deleteContainersAndImages() (*pb.CommandResponse, error) {
	log.Info("Starting deleteContainersAndImages")

	if err := checkDockerStatus(); err != nil {
		log.Error("Docker daemon is not running or accessible", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Docker daemon is not running or accessible: %v", err),
		}, nil
	}

	log.Info("Stopping and removing containers")
	downOutput, err := runDockerCompose("down")
	if err != nil {
		log.Error("Error stopping containers", "error", err, "output", downOutput)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error stopping containers: %v", err),
		}, nil
	}

	runningContainers, err := getRunningContainers()
	if err != nil {
		log.Error("Error checking for running containers", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error checking for running containers: %v", err),
		}, nil
	}

	if len(runningContainers) > 0 {
		log.Warn("Containers still running, force stopping", "containers", runningContainers)
		for _, container := range runningContainers {
			cmdOutput, err := exec.Command("docker", "stop", container).CombinedOutput()
			if err != nil {
				log.Error("Error force stopping container",
					"container", container,
					"error", err,
					"output", string(cmdOutput))
				return &pb.CommandResponse{
					Success: false,
					ErrorMessage: fmt.Sprintf("Error force stopping container %s: %v\nOutput: %s",
						container, err, cmdOutput),
				}, nil
			}
			log.Info("Container force stopped",
				"container", container,
				"output", string(cmdOutput))
		}
	}

	// Remove Gitea images
	if err := removeImages("gitea/gitea"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Remove Postgres images
	if err := removeImages("postgres:13"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

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
		return nil, fmt.Errorf("Error listing running containers: %v", err)
	}
	return strings.Fields(string(output)), nil
}

func removeImages(imageName string) error {
	log.Info("Removing images", "image", imageName)
	images, err := exec.Command("docker", "images", imageName, "-q").Output()
	if err != nil {
		log.Error("Error listing images", "image", imageName, "error", err)
		return fmt.Errorf("Error listing %s images: %v", imageName, err)
	}

	if len(images) > 0 {
		cmd := exec.Command("docker", "rmi", "-f", strings.TrimSpace(string(images)))
		cmdOutput, err := cmd.CombinedOutput()
		if err != nil {
			log.Error("Error removing images",
				"image", imageName,
				"error", err,
				"output", string(cmdOutput))
			return fmt.Errorf("Error removing %s images: %v\nOutput: %s",
				imageName, err, cmdOutput)
		}
		log.Info("Images removed successfully",
			"image", imageName,
			"output", string(cmdOutput))
	}

	return nil
}

func deleteVolumes() (*pb.CommandResponse, error) {
	_, err := runDockerCompose("down", "-v")
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error deleting volumes: %v", err),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Volumes have been deleted.",
	}, nil
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
		logger.Debug("Received message", "type", msgType)

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

		logger.Debug("Sending response", "type", msgType)
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

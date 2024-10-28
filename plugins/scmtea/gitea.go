package main

import (
    "bufio"
    "fmt"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "time"

    "github.com/ssotops/gitspace-plugin-sdk/gsplug"
    "github.com/ssotops/gitspace-plugin-sdk/logger"
    pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

// setupGitea handles the complete Gitea setup process
func (p *ScmteaPlugin) setupGitea(req *pb.CommandRequest) (*pb.CommandResponse, error) {
    progress := gsplug.NewProgressEmitter(func(resp *pb.CommandResponse) error {
        return gsplug.WriteMessage(os.Stdout, resp)
    })

    // Check Docker Compose file first
    if _, err := getComposePath(); err != nil {
        return &pb.CommandResponse{
            Success: false,
            Result: `Before proceeding with setup, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu to continue.`,
            ErrorMessage: "Docker Compose file required. Please select 'Set Docker Compose File' from the Installation menu.",
            Navigation: &pb.NavigationContext{
                CurrentMenu: "installation_menu",
                ParentMenu:  "main",
                AvailableCommands: []*pb.MenuItem{
                    {
                        Label:   "Set Docker Compose File",
                        Command: "set_compose_file",
                    },
                    {
                        Label:   "Enter Custom Docker Compose Path",
                        Command: "set_compose_file_custom",
                        Parameters: []*pb.ParameterInfo{
                            {
                                Name:        "custom_path",
                                Description: "Path to custom Docker Compose file",
                                Required:    true,
                            },
                        },
                    },
                    {
                        Label:   "Go Back",
                        Command: "go_back",
                    },
                },
            },
        }, nil
    }

    // Check Docker status
    progress.Emit("prereq", "docker", "running", "Checking Docker status")
    if err := checkDockerRunning(); err != nil {
        progress.Emit("prereq", "docker", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Docker check failed: %v", err),
            Navigation: &pb.NavigationContext{
                CurrentMenu: "lifecycle_menu",
                ParentMenu:  "main",
                AvailableCommands: []*pb.MenuItem{
                    {
                        Label:   "Start Docker",
                        Command: "start_docker",
                    },
                    {
                        Label:   "Go Back",
                        Command: "go_back",
                    },
                },
            },
        }, nil
    }
    progress.Emit("prereq", "docker", "success", "Docker is running")

    // Check Node.js
    progress.Emit("prereq", "nodejs", "running", "Checking Node.js installation")
    if err := checkNodeJS(); err != nil {
        progress.Emit("prereq", "nodejs", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Node.js check failed: %v", err),
            Result:       "Please install Node.js version 14 or higher before proceeding.",
        }, nil
    }
    progress.Emit("prereq", "nodejs", "success", "Node.js requirements met")

    // Check ports
    progress.Emit("prereq", "ports", "running", "Checking port availability")
    if err := checkRequiredPorts(); err != nil {
        progress.Emit("prereq", "ports", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Port check failed: %v", err),
        }, nil
    }
    progress.Emit("prereq", "ports", "success", "Required ports are available")

    // Start containers
    progress.Emit("startup", "containers", "running", "Starting Docker containers")
    if _, err := runDockerCompose(p, "up", "-d"); err != nil {
        progress.Emit("startup", "containers", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to start containers: %v", err),
            Navigation: &pb.NavigationContext{
                CurrentMenu: "lifecycle_menu",
                ParentMenu:  "main",
                AvailableCommands: []*pb.MenuItem{
                    {
                        Label:   "View Logs",
                        Command: "view_logs",
                    },
                    {
                        Label:   "Try Again",
                        Command: "setup",
                        Parameters: []*pb.ParameterInfo{
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
        }, nil
    }
    progress.Emit("startup", "containers", "success", "Containers started successfully")

    // Wait for Gitea
    progress.Emit("startup", "gitea", "running", "Waiting for Gitea to become available")
    if err := waitForGitea(p.logger); err != nil {
        progress.Emit("startup", "gitea", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Error waiting for Gitea: %v", err),
            Navigation: &pb.NavigationContext{
                CurrentMenu: "lifecycle_menu",
                ParentMenu:  "main",
                AvailableCommands: []*pb.MenuItem{
                    {
                        Label:   "View Logs",
                        Command: "view_logs",
                    },
                    {
                        Label:   "Restart Gitea",
                        Command: "restart",
                    },
                },
            },
        }, nil
    }
    progress.Emit("startup", "gitea", "success", "Gitea is now available")

    // Configure Gitea
    progress.Emit("config", "admin", "running", "Setting up admin account")
    if err := configureGitea(req.Parameters); err != nil {
        progress.Emit("config", "admin", "error", err.Error())
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to configure Gitea: %v", err),
            Navigation: &pb.NavigationContext{
                CurrentMenu: "installation_menu",
                ParentMenu:  "main",
                AvailableCommands: []*pb.MenuItem{
                    {
                        Label:   "Try Again",
                        Command: "setup",
                        Parameters: []*pb.ParameterInfo{
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
        }, nil
    }
    progress.Emit("config", "admin", "success", "Admin account configured successfully")

    return &pb.CommandResponse{
        Success: true,
        Result:  "Gitea setup completed successfully",
        Navigation: &pb.NavigationContext{
            CurrentMenu: "installation_menu",
            ParentMenu:  "main",
            AvailableCommands: []*pb.MenuItem{
                {
                    Label:   "Generate and Upload SSH Key",
                    Command: "generate_ssh_key",
                    Parameters: []*pb.ParameterInfo{
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
    }, nil
}

func waitForGitea(logger *logger.RateLimitedLogger) error {
    client := &http.Client{Timeout: 1 * time.Second}
    maxAttempts := 120 // 2 minutes

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        logger.Debug("Checking Gitea availability",
            "attempt", attempt,
            "maxAttempts", maxAttempts)

        resp, err := client.Get("http://localhost:3000/")
        if err != nil {
            logger.Debug("Connection failed",
                "attempt", attempt,
                "error", err)
            time.Sleep(1 * time.Second)
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode == http.StatusOK {
            logger.Info("Gitea is now available")
            return nil
        }

        logger.Debug("Gitea not ready",
            "attempt", attempt,
            "status", resp.StatusCode)
        time.Sleep(1 * time.Second)
    }

    return fmt.Errorf("Gitea did not become available within %d seconds", maxAttempts)
}

func configureGitea(params map[string]string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("failed to get home directory: %v", err)
    }

    setupScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "setup_gitea.js")
    if _, err := os.Stat(setupScriptPath); os.IsNotExist(err) {
        return fmt.Errorf("setup_gitea.js not found at %s", setupScriptPath)
    }

    cmd := exec.Command("node", setupScriptPath,
        params["username"],
        params["email"],
        params["password"])

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdout pipe: %v", err)
    }

    stderr, err := cmd.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %v", err)
    }

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start setup script: %v", err)
    }

    // Handle stdout in a goroutine
    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            fmt.Printf("Setup output: %s\n", scanner.Text())
        }
    }()

    // Handle stderr in a goroutine
    go func() {
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            fmt.Printf("Setup error: %s\n", scanner.Text())
        }
    }()

    if err := cmd.Wait(); err != nil {
        return fmt.Errorf("setup script failed: %v", err)
    }

    return nil
}

// checkPort verifies if a port is available
func checkPort(port int) error {
    addr := fmt.Sprintf(":%d", port)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("port %d is not available: %v", port, err)
    }
    listener.Close()
    return nil
}

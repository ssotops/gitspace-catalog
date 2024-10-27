package main

import (
	"encoding/json"
	"fmt"

	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

func (p *ScmteaPlugin) GetPluginInfo(req *pb.PluginInfoRequest) (*pb.PluginInfo, error) {
	p.logger.Info("GetPluginInfo called")
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
	p.logger.Info("Executing command",
		"command", req.Command,
		"parameters", req.Parameters)

	switch req.Command {
	case "set_compose_file":
		return &pb.CommandResponse{
			Success: true,
			Result: `Please choose one of the following options:
1. Use Default Docker Compose File
2. Enter Custom Docker Compose Path
3. Go Back`,
			Navigation: &pb.NavigationContext{
				CurrentMenu: "set_compose_file",
				ParentMenu:  "installation_menu",
				AvailableCommands: []*pb.MenuItem{
					{
						Label:   "Use Default Docker Compose File",
						Command: "set_compose_file_default",
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

	case "set_compose_file_default":
		return setComposeFile(p, "Use default", "")

	case "set_compose_file_custom":
		customPath, ok := req.Parameters["custom_path"]
		if !ok || customPath == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required for set_compose_file_custom command",
			}, nil
		}
		return setComposeFile(p, "Enter custom path", customPath)

	case "start":
		return runDockerCompose(p, "up", "-d")

	case "stop":
		return runDockerCompose(p, "down")

	case "setup":
		return p.handleSetup(req)

	case "generate_ssh_key":
		return p.handleGenerateSSHKey(req)

	case "restart":
		return runDockerCompose(p,"restart")

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
		return deleteContainersAndImages(p)

	case "delete_volumes":
		return deleteVolumes(p)

	case "go_back":
		return &pb.CommandResponse{
			Success: true,
			Result:  "Returned to previous menu",
			Navigation: &pb.NavigationContext{
				ParentMenu: "main",
			},
		}, nil

	case "configure_backup":
		if err := validateBackupConfig(req.Parameters); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Invalid backup configuration: %v", err),
			}, nil
		}
		return p.handleBackupCommands(req)

	case "create_backup":
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before creating a backup, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu.`,
				ErrorMessage: "Docker Compose file required for backup operations.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
					},
				},
			}, nil
		}
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
		p.logger.Error("Unknown command received", "command", req.Command)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Unknown command: %s", req.Command),
		}, nil
	}
}

func (p *ScmteaPlugin) handleSetup(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	// Check for Docker Compose file before proceeding
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
						Label:   "Go Back",
						Command: "go_back",
					},
				},
			},
		}, nil
	}

	// Validate parameters
	for _, param := range []string{"username", "password", "email"} {
		if _, ok := req.Parameters[param]; !ok {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
			}, nil
		}
	}

	return p.setupGitea(req)
}

func (p *ScmteaPlugin) handleGenerateSSHKey(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	// Validate parameters
	for _, param := range []string{"username", "password", "email"} {
		if _, ok := req.Parameters[param]; !ok {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
			}, nil
		}
	}
	return generateAndUploadSSHKey(p, req)
}

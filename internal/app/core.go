package app

import (
	"fmt"
	"log"
	"os"

	"github.com/mako10k/llmcmd/internal/cli"
	"github.com/mako10k/llmcmd/internal/install"
	"github.com/mako10k/llmcmd/internal/openai"
)

// ApplicationMetadata contains application version information
type ApplicationMetadata struct {
	Name    string
	Version string
}

// ExecutionContext contains runtime context for llmcmd execution
type ExecutionContext struct {
	IsInternal  bool                       // true if called from llmsh
	SharedQuota *openai.SharedQuotaManager // shared quota manager (if internal)
	ProcessID   string                     // process ID for quota tracking
	ParentID    string                     // parent process ID
}

// LLMCmdCore represents the core llmcmd functionality
type LLMCmdCore struct {
	metadata ApplicationMetadata
	context  *ExecutionContext
}

// NewLLMCmdCore creates a new core instance
func NewLLMCmdCore(metadata ApplicationMetadata, context *ExecutionContext) *LLMCmdCore {
	return &LLMCmdCore{
		metadata: metadata,
		context:  context,
	}
}

// ExecuteWithArgs executes llmcmd with provided arguments
func (core *LLMCmdCore) ExecuteWithArgs(args []string) error {
	// Parse command line arguments
	config, err := cli.ParseArgs(args)
	if err != nil {
		switch err {
		case cli.ErrShowHelp:
			cli.ShowHelp()
			return nil
		case cli.ErrShowVersion:
			fmt.Printf("%s version %s\n", core.metadata.Name, core.metadata.Version)
			return nil
		case cli.ErrListPresets:
			return core.handleListPresets(config)
		case cli.ErrInstall:
			return core.handleInstall()
		default:
			return fmt.Errorf("argument parsing error: %w", err)
		}
	}

	// Load and merge configuration
	mergedConfig, err := cli.LoadAndMergeConfig(config)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Model selection priority: user-facing calls use main model, internal calls use internal model
	if core.context.IsInternal && mergedConfig.InternalModel != "" {
		// Internal call (llmcmd → llmsh → llmcmd): use internal model
		mergedConfig.Model = mergedConfig.InternalModel
	}
	// User-facing calls (standalone llmcmd/llmsh): keep main model setting

	// Resolve preset if specified
	finalPrompt, err := core.resolvePrompt(config, mergedConfig)
	if err != nil {
		return fmt.Errorf("prompt resolution error: %w", err)
	}
	config.Prompt = finalPrompt

	// Initialize logging
	core.setupLogging(config)

	// Create application with appropriate quota manager
	var app *App
	if core.context.IsInternal && core.context.SharedQuota != nil {
		// Internal call: use shared quota
		app = NewWithSharedQuota(config, core.context.SharedQuota, core.context.ProcessID, core.metadata)
	} else {
		// External call: create new quota
		app = New(config)
	}

	// Run the application
	if err := app.Run(); err != nil {
		return fmt.Errorf("application error: %w", err)
	}

	// Handle exit request if needed
	if app.IsExitRequested() {
		if core.context.IsInternal {
			// Internal calls should not exit the parent process
			return fmt.Errorf("exit requested with code %d", app.GetExitCode())
		} else {
			// External calls can exit
			os.Exit(app.GetExitCode())
		}
	}

	return nil
}

// handleListPresets handles the --list-presets option
func (core *LLMCmdCore) handleListPresets(config *cli.Config) error {
	// Start with default configuration
	defaultConfig := cli.DefaultConfig()

	// If a config file is specified, try to load and merge it
	if config.ConfigFile != "" {
		fileConfig, loadErr := cli.LoadConfigFile(config.ConfigFile, true) // Explicitly specified
		if loadErr == nil && fileConfig.PromptPresets != nil {
			// Merge file config with defaults (presets from file take precedence)
			for k, v := range fileConfig.PromptPresets {
				defaultConfig.PromptPresets[k] = v
			}
		}
	}

	fmt.Println("Available prompt presets:")
	for key, preset := range defaultConfig.PromptPresets {
		fmt.Printf("  %-12s - %s\n", key, preset.Description)
	}
	return nil
}

// handleInstall handles the --install option
func (core *LLMCmdCore) handleInstall() error {
	installer := install.NewSystemInstaller(true)
	if err := installer.Install(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	return nil
}

// resolvePrompt resolves the final prompt from config and presets
func (core *LLMCmdCore) resolvePrompt(config *cli.Config, mergedConfig *cli.ConfigFile) (string, error) {
	finalPrompt := config.Prompt
	if config.Preset != "" {
		if config.Prompt != "" {
			return "", fmt.Errorf("cannot specify both --prompt and --preset options")
		}

		presetContent, err := cli.ResolvePreset(mergedConfig, config.Preset)
		if err != nil {
			return "", fmt.Errorf("preset resolution error: %w", err)
		}
		finalPrompt = presetContent
	} else if config.Prompt == "" {
		// Use default preset if no prompt specified
		defaultPreset := mergedConfig.DefaultPrompt
		if defaultPreset != "" {
			presetContent, err := cli.ResolvePreset(mergedConfig, defaultPreset)
			if err != nil {
				log.Printf("Warning: Could not resolve default preset '%s': %v", defaultPreset, err)
			} else {
				finalPrompt = presetContent
			}
		}
	}
	return finalPrompt, nil
}

// setupLogging configures logging based on config
func (core *LLMCmdCore) setupLogging(config *cli.Config) {
	if config.Verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(os.Stderr)
		log.SetFlags(0)
	}
}

// ExecuteExternal executes llmcmd as an external command (standalone)
func ExecuteExternal(metadata ApplicationMetadata, args []string) error {
	context := &ExecutionContext{
		IsInternal:  false,
		SharedQuota: nil,
		ProcessID:   "",
		ParentID:    "",
	}

	core := NewLLMCmdCore(metadata, context)
	return core.ExecuteWithArgs(args)
}

// ExecuteInternal executes llmcmd as an internal command (from llmsh)
func ExecuteInternal(metadata ApplicationMetadata, args []string, sharedQuota *openai.SharedQuotaManager, processID, parentID string) error {
	context := &ExecutionContext{
		IsInternal:  true,
		SharedQuota: sharedQuota,
		ProcessID:   processID,
		ParentID:    parentID,
	}

	core := NewLLMCmdCore(metadata, context)
	return core.ExecuteWithArgs(args)
}

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mako10k/llmcmd/internal/app"
	"github.com/mako10k/llmcmd/internal/cli"
	"github.com/mako10k/llmcmd/internal/install"
)

// Application metadata
const (
	AppName    = "llmcmd"
	AppVersion = "3.0.3"
)

func main() {
	// Parse command line arguments
	config, err := cli.ParseArgs(os.Args[1:])
	if err != nil {
		switch err {
		case cli.ErrShowHelp:
			cli.ShowHelp()
			os.Exit(0)
		case cli.ErrShowVersion:
			fmt.Printf("%s version %s\n", AppName, AppVersion)
			os.Exit(0)
		case cli.ErrListPresets:
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
			os.Exit(0)
		case cli.ErrInstall:
			installer := install.NewSystemInstaller(true)
			if err := installer.Install(); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Load and merge configuration
	mergedConfig, err := cli.LoadAndMergeConfig(config)
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Resolve preset if specified
	finalPrompt := config.Prompt
	if config.Preset != "" {
		if config.Prompt != "" {
			log.Fatal("Error: Cannot specify both --prompt and --preset options")
		}

		presetContent, err := cli.ResolvePreset(mergedConfig, config.Preset)
		if err != nil {
			log.Fatalf("Preset resolution error: %v", err)
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

	// Update config with resolved prompt
	config.Prompt = finalPrompt

	// Initialize logging
	if config.Verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(os.Stderr)
		log.SetFlags(0)
	}

	// Run the main application
	app := app.New(config)
	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	// Check if exit tool was called and exit with the requested code
	if app.IsExitRequested() {
		os.Exit(app.GetExitCode())
	}
}

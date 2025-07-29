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
	AppVersion = "1.0.0-dev"
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

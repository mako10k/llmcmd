package main

import (
	"log"
	"os"

	"github.com/mako10k/llmcmd/internal/app"
)

// Application metadata
var (
	AppName    = "llmcmd"
	AppVersion = "3.1.1" // Will be overridden by build-time ldflags
)

func main() {
	metadata := app.ApplicationMetadata{
		Name:    AppName,
		Version: AppVersion,
	}

	// Execute as external command
	if err := app.ExecuteExternal(metadata, os.Args[1:]); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

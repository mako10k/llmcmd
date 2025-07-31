package main

import (
	"log"
	"os"

	"github.com/mako10k/llmcmd/internal/app"
)

// Application metadata
const (
	AppName    = "llmcmd"
	AppVersion = "3.0.3"
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

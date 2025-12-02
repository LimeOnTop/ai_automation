package main

import (
	"fmt"
	"os"

	"ai_automation/presentation/terminal"
)

func main() {
	termInterface, err := terminal.NewTerminalInterface()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer termInterface.Close()

	if err := termInterface.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}


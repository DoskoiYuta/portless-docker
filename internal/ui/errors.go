package ui

import (
	"fmt"
	"os"
	"os/exec"
)

// CheckDockerCompose verifies that docker compose is available.
func CheckDockerCompose() error {
	_, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker compose is not installed or not in PATH.")
	}

	// Check that 'docker compose' subcommand works.
	cmd := exec.Command("docker", "compose", "version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose is not installed or not in PATH.")
	}

	return nil
}

// Fatal prints an error and exits with code 1.
func Fatal(msg string) {
	PrintError(msg)
	os.Exit(1)
}

// FatalErr prints an error from an error value and exits with code 1.
func FatalErr(err error) {
	PrintError(err.Error())
	os.Exit(1)
}

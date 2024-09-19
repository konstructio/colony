package exec

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/konstructio/colony/internal/logger"
)

// ExecuteCommand runs a shell command and returns its output as a string.
// It logs the command execution and any warnings, and returns an error if the command fails.
func ExecuteCommand(logger *logger.Logger, command string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Debugf("Executing command: %s %s", command, strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error executing command: %w, stderr: %s", err, stderr.String())
	}

	if stderr.Len() > 0 {
		logger.Warnf("Command executed with warnings: %s", stderr.String())
	}

	output := stdout.String()
	logger.Debugf("Command output: %s", output)

	return output, nil
}

package osutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// UserExists checks if the given username exists on the system.
// Returns true if the user exists, false otherwise, along with any error.
func UserExists(username string) (bool, error) {
	cmd := exec.Command("id", username)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("failed to check user %s: %v", username, err)
}

// AddUser creates a system user with no password, no login shell, and the specified home directory.
// It ensures the home directory exists, is owned by the user, and has 700 permissions.
// If password is empty, no password is set (disabling password login).
func AddUser(username, password, homedir string) error {
	// Check if user already exists
	exists, err := UserExists(username)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %v", err)
	}
	if exists {
		return fmt.Errorf("user %s already exists", username)
	}

	// Create the user with no login shell and specified home directory
	args := []string{"-r", "-s", "/sbin/nologin", "-m", "-d", homedir, username}
	cmd := exec.Command("useradd", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create user %s: %v, output: %s", username, err, strings.TrimSpace(string(output)))
	}

	// Ensure home directory ownership
	cmd = exec.Command("chown", fmt.Sprintf("%s:%s", username, username), homedir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set ownership of %s: %v, output: %s", homedir, err, strings.TrimSpace(string(output)))
	}

	// Set home directory permissions to 700
	cmd = exec.Command("chmod", "700", homedir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %v, output: %s", homedir, err, strings.TrimSpace(string(output)))
	}

	// If a password is provided, set it (though typically empty for system users)
	if password != "" {
		cmd := exec.Command("passwd", username)
		cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to set password for %s: %v, output: %s", username, err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

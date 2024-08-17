package wgroot

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// Launch a copy of this process, but with the --kernelwg command line argument.
// This other process will run with root privileges, because it needs to be able to
// create and/or alter Wireguard interfaces.
//
// This is one of the first things we do when starting up the cyclops server
// or the HTTPS proxy server.
//
// Returns a secret that is used to authenticate ourselves to the root-mode
// spawned process.
func LaunchRootModeSubProcess() (e error, secret string) {
	if syscall.Getuid() != 0 {
		return fmt.Errorf("Must be root to launch kernel wireguard interface."), ""
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to find own executable path: %v", err), ""
	}

	fmt.Printf("Launching wireguard root-mode sub-process %v\n", self)

	secret = strongRandomBase64(32)
	cmd := exec.Command(self, "--kernelwg")
	cmd.Env = append(cmd.Env, fmt.Sprintf("CYCLOPS_SOCKET_SECRET=%v", secret))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%v", os.Getenv("PATH")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to start sub-process: %v", err), ""
	}

	return nil, secret
}

// Drop privileges of this process to the specified username, so that we reduce our attack surface.
// Returns the home directory of 'username'
func DropPrivileges(username string) error {
	if username == "" {
		return fmt.Errorf("You must specify username")
	}
	userRec, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("Failed to find user '%v': %v", username, err)
	}

	uid, _ := strconv.Atoi(userRec.Uid)
	gid, _ := strconv.Atoi(userRec.Gid)

	fmt.Printf("Dropping privileges (becoming user '%v', uid:%v, gid:%v)\n", username, uid, gid)

	// It's important to change group before user

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("Failed to setgid: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("Failed to setuid: %v", err)
	}
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		return fmt.Errorf("Failed to setresuid: %v", err)
	}
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		return fmt.Errorf("Failed to setresgid: %v", err)
	}

	os.Setenv("HOME", userRec.HomeDir)

	return nil
}

// Return true if we are running as the given username
func IsRunningAsUser(username string) bool {
	userRec, err := user.Lookup(username)
	if err != nil {
		return false
	}
	uid, _ := strconv.Atoi(userRec.Uid)
	return syscall.Getuid() == uid
}

// This is used after dropping privileges, to make sure that our process has all the hallmarks
// of a normal user process. The reason this was created was so that NCNN could read from
// /proc/self/auxv to detect CPU features.
func RelaunchSelf(args, env []string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("Failed to find own executable path: %v", err)
	}

	fmt.Printf("Relaunching %v with reduced privileges\n", self)

	cmd := exec.Command(self, args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%v", os.Getenv("PATH")))
	cmd.Env = append(cmd.Env, fmt.Sprintf("HOME=%v", os.Getenv("HOME")))
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Failed to start sub-process: %v", err)
	}

	return cmd, nil
}

func strongRandomBase64(nbytes int) string {
	buf := make([]byte, nbytes)
	if n, _ := rand.Read(buf[:]); n != nbytes {
		panic("Unable to read from crypto/rand")
	}
	return base64.StdEncoding.EncodeToString(buf)
}

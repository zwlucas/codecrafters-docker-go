package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	tmpDir := "/tmp/mydocker"

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	_ = os.Mkdir(tmpDir, 0755)

	err := exec.Command("mkdir", "-p", filepath.Join(tmpDir, filepath.Dir(command))).Run()
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	err = exec.Command("cp", command, filepath.Join(tmpDir, command)).Run()
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: tmpDir,
	}

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		} else {
			fmt.Printf("Err: %v", err)
			os.Exit(1)
		}
	}
}

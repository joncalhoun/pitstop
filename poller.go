package pitstop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DidChange will scan the provided directory looking for any files that have
// changed after the provided `since` time.Time. If one is found, true is
// returned. Otherwise false is returned.
func DidChange(dir string, since time.Time) bool {
	var changed bool

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().After(since) {
			changed = true
		}
		return nil
	})

	return changed
}

// BuildFunc is a function that performs a build step. This might be something
// like copying files, running an exec.Cmd, or something else entirely.
type BuildFunc func() error

// BuildCommand works similar to exec.Command, but rather than returning an
// exec.Cmd it returns a BuildFunc that can be reused.
func BuildCommand(command string, args ...string) BuildFunc {
	return func() error {
		cmd := exec.Command(command, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("error building: \"%s %s\": %w", command, strings.Join(args, " "), err)
		}
		return nil
	}
}

// RunFunc is a function that runs an application asynchronously and returns a
// function to stop the app.
type RunFunc func() (stop func(), err error)

// RunCommand works similar to exec.Command, but rather than returning an
// exec.Cmd it returns a RunFunc that can be reused.
func RunCommand(command string, args ...string) RunFunc {
	return func() (func(), error) {
		cmd := exec.Command(command, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			return nil, fmt.Errorf("error running: \"%s %s\": %w", command, strings.Join(args, " "), err)
		}
		return func() {
			cmd.Process.Kill()
		}, nil
	}
}

// Run will run all pre BuildFuncs, then the RunFunc, and then finally the post
// BuildFuncs. Any errors encountered will be returned, and the build process
// halted. If RunFunc has been called, stop will also be called so that it is
// guaranteed to not be running anytime an error is returned.
func Run(pre []BuildFunc, run RunFunc, post []BuildFunc) (func(), error) {
	for _, fn := range pre {
		err := fn()
		if err != nil {
			return nil, err
		}
	}
	stop, err := run()
	if err != nil {
		return nil, err
	}
	for _, fn := range post {
		err := fn()
		if err != nil {
			stop()
			return nil, err
		}
	}
	return stop, nil
}

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

// Poller is used to poll a directory and its subdirectories for changes, and
// then will kick off a rebuild of the app when changes are detected.
type Poller struct {
	// ScanInterval is the duration of time the poller will wait before scanning for new file changes. This defaults to 500ms.
	ScanInterval time.Duration

	// Dir is the directory to scan for file changes. This defaults to "." if it isn't provided.
	Dir string

	// Pre, Run, and Post represent the functions used to build and run our app.
	// Pre functions are called first, then run, then finally the post functions.
	Pre  []BuildFunc
	Run  RunFunc
	Post []BuildFunc
}

// Poll is a long running process that continuously scans for changes and
// then runs the build and run functions when changes are detected.
func (p *Poller) Poll() {
	scanInt := p.ScanInterval
	if scanInt == 0 {
		scanInt = 500 * time.Millisecond
	}
	dir := p.Dir
	if dir == "" {
		dir = "."
	}

	var stop func()
	var err error
	var lastBuild time.Time

	for {
		if !DidChange(p.Dir, lastBuild) {
			time.Sleep(scanInt)
			continue
		}
		if stop != nil {
			fmt.Println("Stopping running app...")
			stop()
		}
		fmt.Println("Building & Running app...")
		stop, err = Run(p.Pre, p.Run, p.Post)
		if err != nil {
			fmt.Printf("Error running: %v\n", err)
		}
		lastBuild = time.Now()
		time.Sleep(scanInt)
	}
}

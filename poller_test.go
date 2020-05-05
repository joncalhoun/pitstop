package pitstop_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joncalhoun/pitstop"
)

func TestDidChange(t *testing.T) {
	removeAllFn := func(dir string) func() {
		return func() {
			os.RemoveAll(dir)
		}
	}

	type testCase struct {
		setup func(t *testing.T) (dir string, since time.Time, teardown func())
		want  bool
	}
	for name, tc := range map[string]testCase{
		"empty": {
			func(t *testing.T) (string, time.Time, func()) {
				dir, err := ioutil.TempDir("", "")
				if err != nil {
					t.Fatalf("setup: creating temp dir: %v", err)
				}
				time := time.Now()
				return dir, time, removeAllFn(dir)
			},
			false,
		},
		"subdir unchanged": {
			func(t *testing.T) (string, time.Time, func()) {
				dir, err := ioutil.TempDir("", "")
				if err != nil {
					t.Fatalf("setup: creating temp dir: %v", err)
				}
				err = os.MkdirAll(filepath.Join(dir, "goes", "a", "few", "layers", "deep"), 0700)
				if err != nil {
					t.Fatalf("setup: creating subdir: %v", err)
				}
				time := time.Now()
				return dir, time, removeAllFn(dir)
			},
			false,
		},
		"flat new file": {
			func(t *testing.T) (string, time.Time, func()) {
				dir, err := ioutil.TempDir("", "")
				if err != nil {
					t.Fatalf("setup: creating temp dir: %v", err)
				}
				time := time.Now()
				_, err = ioutil.TempFile(dir, "")
				if err != nil {
					t.Fatalf("setup: creating new file: %v", err)
				}
				return dir, time, removeAllFn(dir)
			},
			true,
		},
		"subdir new file": {
			func(t *testing.T) (string, time.Time, func()) {
				dir, err := ioutil.TempDir("", "")
				if err != nil {
					t.Fatalf("setup: creating temp dir: %v", err)
				}
				subdir := filepath.Join(dir, "goes", "a", "few", "layers", "deep")
				err = os.MkdirAll(subdir, 0700)
				if err != nil {
					t.Fatalf("setup: creating subdir: %v", err)
				}
				time := time.Now()
				_, err = ioutil.TempFile(subdir, "")
				if err != nil {
					t.Fatalf("setup: creating new file: %v", err)
				}
				return dir, time, removeAllFn(dir)
			},
			true,
		},
		"subdir changed file": {
			func(t *testing.T) (string, time.Time, func()) {
				dir, err := ioutil.TempDir("", "")
				if err != nil {
					t.Fatalf("setup: creating temp dir: %v", err)
				}
				subdir := filepath.Join(dir, "goes", "a", "few", "layers", "deep")
				err = os.MkdirAll(subdir, 0700)
				if err != nil {
					t.Fatalf("setup: creating subdir: %v", err)
				}
				f, err := ioutil.TempFile(subdir, "")
				if err != nil {
					t.Fatalf("setup: creating new file: %v", err)
				}
				time := time.Now()
				fmt.Fprintln(f, "this is a change")
				return dir, time, removeAllFn(dir)
			},
			true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, since, teardown := tc.setup(t)
			defer teardown()
			got := pitstop.DidChange(dir, since)
			if got != tc.want {
				t.Errorf("DidChange() = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	buildCommand := func(cmd string, args ...string) func(*testing.T) []pitstop.BuildFunc {
		return func(*testing.T) []pitstop.BuildFunc {
			return []pitstop.BuildFunc{pitstop.BuildCommand(cmd, args...)}
		}
	}
	runCommand := func(cmd string, args ...string) func(t *testing.T) pitstop.RunFunc {
		return func(*testing.T) pitstop.RunFunc {
			return pitstop.RunCommand(cmd, args...)
		}
	}
	errorOnBuild := func(msg string) func(*testing.T) []pitstop.BuildFunc {
		return func(t *testing.T) []pitstop.BuildFunc {
			return []pitstop.BuildFunc{
				func() error {
					t.Error(msg)
					return nil
				},
			}
		}
	}
	errorOnRun := func(msg string) func(*testing.T) pitstop.RunFunc {
		return func(t *testing.T) pitstop.RunFunc {
			return func() (func(), error) {
				t.Error(msg)
				return func() {}, nil
			}
		}
	}

	type testCase struct {
		pre  func(*testing.T) []pitstop.BuildFunc
		run  func(*testing.T) pitstop.RunFunc
		post func(*testing.T) []pitstop.BuildFunc
		err  bool
	}
	for name, tc := range map[string]testCase{
		"tail": {
			run: runCommand("tail"),
			err: false,
		},
		"exit 1": {
			run: runCommand("exit", "1"),
			err: true,
		},
		"all good": {
			pre:  buildCommand("echo", "hello"),
			run:  runCommand("tail"),
			post: buildCommand("echo", "goodbye"),
		},
		"run and post aren't called on pre error": {
			pre:  buildCommand("exit", "1"),
			run:  errorOnRun("run shouldn't be called after a pre error"),
			post: errorOnBuild("post shouldn't be called after a pre error"),
			err:  true,
		},
		"post isnt called on run error": {
			run:  runCommand("exit", "1"),
			post: errorOnBuild("post shouldn't be called after a run error"),
			err:  true,
		},
		"chain error": {
			pre: func(t *testing.T) []pitstop.BuildFunc {
				return []pitstop.BuildFunc{
					pitstop.BuildCommand("echo", "hi"),
					pitstop.BuildCommand("exit", "1"),
					func() error {
						t.Errorf("additional pre commands shouldn't be run after an error")
						return nil
					},
				}
			},
			err: true,
		},
		// In an ideal world we would validate that the stop func is called
		// after post errors, but I'm feeling lazy right now.
		"error in post": {
			pre:  buildCommand("echo", "hello"),
			run:  runCommand("tail"),
			post: buildCommand("exit", "1"),
			err:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var pre, post []pitstop.BuildFunc
			var run pitstop.RunFunc
			if tc.pre != nil {
				pre = tc.pre(t)
			}
			if tc.run != nil {
				run = tc.run(t)
			}
			if tc.post != nil {
				post = tc.post(t)
			}
			stop, err := pitstop.Run(pre, run, post)
			if err != nil {
				if !tc.err {
					t.Errorf("Run() err = %v; wanted no errors", err)
				}
				return
			}
			if tc.err {
				t.Errorf("Run() err = nill; wanted an error")
			}
			stop()
		})
	}
}

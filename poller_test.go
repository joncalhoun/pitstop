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

package pitstop

import (
	"os"
	"path/filepath"
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

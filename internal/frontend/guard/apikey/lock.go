package apikey

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// lockWait bounds how long a Mint or Revoke waits for another to finish with
// the file, and lockPoll how often it asks.
var (
	lockWait = 5 * time.Second
	lockPoll = 20 * time.Millisecond
)

// lock claims the file for one Mint or Revoke by creating path.lock exclusively, so
// two edits at once do not undo each other; a lock left by a process that died is named.
func lock(path string) (unlock func(), err error) {
	name := path + ".lock"
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- beside the file the operator named
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(name) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%s is being edited by another mint or revoke; if none is running, remove %s", path, name)
		}
		time.Sleep(lockPoll)
	}
}

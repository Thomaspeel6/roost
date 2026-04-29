package events

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var appendMu sync.Mutex

// Path returns the events log path. Honors ROOST_EVENTS_PATH for tests/overrides.
func Path() string {
	if p := os.Getenv("ROOST_EVENTS_PATH"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".roost", "events.jsonl")
}

// Append writes one event atomically.
//
// Concurrency model:
//   - Within one process: a sync.Mutex serializes writers.
//   - Across processes: O_APPEND guarantees that each write(2) syscall is
//     atomic at the file-offset level on POSIX, and Windows similarly
//     atomically appends with FILE_APPEND_DATA. We marshal each Event to a
//     single byte slice (json + newline) and write it in one Write call, so
//     concurrent appends from multiple roost-hook processes never interleave.
//
// Errors are returned, but the caller (roost-hook) should never block CC on
// them — logging to stderr and exiting 0 is the right behavior at the boundary.
func Append(e Event) error {
	if e.Schema == 0 {
		e.Schema = SchemaVersion
	}
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	buf, err := json.Marshal(e)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')

	appendMu.Lock()
	defer appendMu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(buf)
	return err
}

// Replay returns up to maxEvents most-recent events from the log,
// silently skipping corrupt lines so a single bad write doesn't poison
// the entire log.
func Replay(maxEvents int) ([]Event, error) {
	path := Path()
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var all []Event
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		all = append(all, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if maxEvents > 0 && len(all) > maxEvents {
		all = all[len(all)-maxEvents:]
	}
	return all, nil
}

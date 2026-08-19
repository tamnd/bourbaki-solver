package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// Where a question goes after it has been asked.
//
// reports/ask-usage.jsonl is to translating, solving and the glossary what
// reports/ocr-usage.jsonl is to reading pages: appended to by every run, never
// replaced, and the only record of how these boxes behave over a week rather
// than during the last hour. report usage reads both.

// One mutex for the process and not one for each recorder. The lanes ask in
// parallel and solve builds a recorder an exercise, so a mutex inside the
// closure would guard each lane against itself and none of them against each
// other.
var askLog sync.Mutex

// noteAsks returns a recorder that appends one line a question. A run that
// cannot open the file says so once and then asks its questions anyway, because
// a report is not worth failing a translation over.
func noteAsks(root string, logf func(string, ...any)) func(ocr.Note) {
	path := filepath.Join(root, "reports", "ask-usage.jsonl")
	var complained bool
	return func(note ocr.Note) {
		line, err := json.Marshal(note)
		if err != nil {
			return
		}
		// An O_APPEND write of a short line is atomic on the systems this runs
		// on, and the mutex costs nothing next to a question that takes a
		// minute, so both are here and neither is relied on alone.
		askLog.Lock()
		defer askLog.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			if !complained && logf != nil {
				complained = true
				logf("the usage log could not be opened, questions will not be recorded: %v", err)
			}
			return
		}
		defer file.Close()
		file.Write(append(line, '\n'))
	}
}

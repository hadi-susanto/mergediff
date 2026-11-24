package dirdiff

import (
	"bytes"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
)

// chunkSize is how much of each file sameFiles reads at a time.
const chunkSize = 64 * 1024

// workers is how many files are read in parallel. Reading is mostly I/O,
// so it uses more goroutines than CPUs.
func workers() int {
	return min(16, max(4, 2*runtime.NumCPU()))
}

// compareContents checks the files of every entry that needs it, in
// parallel. Modified entries with equal sizes become Unchanged when their
// bytes are equal. Every other file is opened once, so an unreadable file
// shows as Error now rather than failing later.
func compareContents(entries []Entry, p *Progress) {
	jobs := make(chan *Entry)
	var wg sync.WaitGroup
	for range workers() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range jobs {
				check(e)
				p.addCompared(1)
			}
		}()
	}
	var settled int64 // entries that need no check
	for i := range entries {
		if entries[i].Note == "" && entries[i].Status != Error && entries[i].Status != Unchanged {
			jobs <- &entries[i]
		} else {
			settled++
		}
	}
	p.addCompared(settled)
	close(jobs)
	wg.Wait()
}

// check settles one entry (see compareContents).
func check(e *Entry) {
	var err error
	switch {
	case e.Status == Modified && e.LeftSize == e.RightSize:
		var same bool
		if same, err = sameFiles(e.Left, e.Right); err == nil && same {
			e.Status = Unchanged
		}
	default:
		for _, p := range []string{e.Left, e.Right} {
			if p != "" && err == nil {
				err = readable(p)
			}
		}
	}
	if err != nil {
		e.Status, e.Note = Error, message(err)
	}
}

// readable reports an error if path cannot be opened for reading.
func readable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// sameFiles reports whether two files have the same bytes, reading both
// a chunk at a time so large files are never loaded whole.
func sameFiles(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	bufA, bufB := make([]byte, chunkSize), make([]byte, chunkSize)
	for {
		na, errA := io.ReadFull(fa, bufA)
		nb, errB := io.ReadFull(fb, bufB)
		if err := readErr(errA); err != nil {
			return false, err
		}
		if err := readErr(errB); err != nil {
			return false, err
		}
		if !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		if errA != nil || errB != nil { // at least one reached its end
			return errA != nil && errB != nil, nil
		}
	}
}

// readErr returns err unless it only marks the end of the file. Read
// errors from os already name the file.
func readErr(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil
	}
	return err
}

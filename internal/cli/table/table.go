// Package table provides output formatting utilities for the oasis CLI.
package table

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

// KVPair holds a key/value pair for display in a vertical table.
type KVPair struct {
	Key   string
	Value string
}

// PrintTable renders a table with a header row and separator to w using tabwriter.
func PrintTable(headers []string, rows [][]string, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Separator line per column.
	seps := make([]string, len(headers))
	for i, h := range headers {
		seps[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(tw, strings.Join(seps, "\t"))

	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	tw.Flush()
}

// PrintKV renders a vertical key/value list using tabwriter.
func PrintKV(pairs []KVPair, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, p := range pairs {
		fmt.Fprintf(tw, "%s\t%s\n", p.Key, p.Value)
	}
	tw.Flush()
}

// spinnerFrames are the animation frames for the terminal spinner.
var spinnerFrames = []string{"|", "/", "-", "\\"}

// isStderrTTY reports whether stderr is a terminal.
func isStderrTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// Spinner shows an animated spinner on stderr while f runs.
// When stderr is not a TTY (e.g. a pipe or CI), the spinner is suppressed.
// Returns the error returned by f.
func Spinner(label string, f func() error) error {
	if !isStderrTTY() {
		return f()
	}

	var (
		mu   sync.Mutex
		done bool
		idx  int
	)

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mu.Lock()
				if !done {
					frame := spinnerFrames[idx%len(spinnerFrames)]
					idx++
					fmt.Fprintf(os.Stderr, "\r%s %s", frame, label)
				}
				mu.Unlock()
			}
		}
	}()

	err := f()

	mu.Lock()
	done = true
	mu.Unlock()
	close(stop)

	// Clear the spinner line.
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", len(label)+3))

	return err
}

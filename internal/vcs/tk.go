package vcs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type tkRunner struct{}

// NewTKRunner returns a TKRunner backed by the real tk binary
// (HERDLE_TK override, else PATH).
func NewTKRunner() TKRunner { return tkRunner{} }

func (tkRunner) tk(dir string, args ...string) (result, error) {
	bin, err := resolveBinary("HERDLE_TK", "tk")
	if err != nil {
		return result{}, err
	}
	return run(dir, bin, args...)
}

// tkQueryRow mirrors the JSON `tk query` emits per line. Priority is a string in
// that output (e.g. "2"); Title is absent (it lives in the ticket body heading).
type tkQueryRow struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Lifecycle   string `json:"lifecycle"`
	Branch      string `json:"branch"`
	ExternalRef string `json:"external-ref"`
	Type        string `json:"type"`
	Assignee    string `json:"assignee"`
	Priority    string `json:"priority"`
}

func (t tkRunner) Tickets(path string) ([]Ticket, error) {
	res, err := t.tk(path, "query")
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("tk query: %s", strings.TrimSpace(res.stderr))
	}
	var tickets []Ticket
	sc := bufio.NewScanner(strings.NewReader(res.stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row tkQueryRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("tk query: parse %q: %w", line, err)
		}
		if row.ID == "" {
			continue // malformed row with no id — don't admit a zero-value Ticket
		}
		// Atoi("") and any non-numeric value yield 0, the correct default.
		pri, _ := strconv.Atoi(row.Priority)
		title, err := ticketTitle(filepath.Join(path, ".tickets", row.ID+".md"))
		if err != nil {
			return nil, fmt.Errorf("tk ticket %s: %w", row.ID, err)
		}
		tickets = append(tickets, Ticket{
			ID: row.ID, Status: row.Status, Lifecycle: row.Lifecycle, Title: title,
			Branch: row.Branch, ExternalRef: row.ExternalRef, Type: row.Type,
			Assignee: row.Assignee, Priority: pri,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tk query: read: %w", err)
	}
	return tickets, nil
}

// ticketTitle returns the first "# " heading in a ticket markdown file, or "" if
// the file is missing or has no heading.
func ticketTitle(mdPath string) (string, error) {
	f, err := os.Open(mdPath) // #nosec G304 -- path is .tickets/<tk-reported-id>.md under the repo
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:]), nil
		}
	}
	return "", sc.Err()
}

func (t tkRunner) Ready(path string) ([]string, error) {
	res, err := t.tk(path, "ready")
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("tk ready: %s", strings.TrimSpace(res.stderr))
	}
	var ids []string
	for _, line := range strings.Split(res.trimmed(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "[") {
			continue // skip blanks and any non-ticket footer line
		}
		ids = append(ids, strings.Fields(line)[0])
	}
	return ids, nil
}

func (t tkRunner) HasTickets(path string) (bool, error) {
	info, err := os.Stat(filepath.Join(path, ".tickets"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

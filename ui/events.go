package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mongodumpx/dbcontext"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Navigation and lifecycle
// ---------------------------------------------------------------------------

type gotoPageMsg struct{ page Page }

func goTo(page Page) tea.Cmd {
	return func() tea.Msg { return gotoPageMsg{page} }
}

type connectedMsg struct{ err error }

func connectCmd(uri string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return connectedMsg{err: dbcontext.Connect(ctx, uri)}
	}
}

type modeChosenMsg struct{ mode Mode }

type startBackupMsg struct{}
type startRestoreMsg struct{}

// ---------------------------------------------------------------------------
// Data loading
// ---------------------------------------------------------------------------

type dbsLoadedMsg struct {
	names []string
	err   error
}

func loadDBsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		names, err := dbcontext.ListDatabases(ctx)
		return dbsLoadedMsg{names: names, err: err}
	}
}

type collsLoadedMsg struct {
	db    string
	names []string
	err   error
}

func loadCollsCmd(db string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		names, err := dbcontext.ListCollections(ctx, db)
		return collsLoadedMsg{db: db, names: names, err: err}
	}
}

type backupScanResultMsg struct {
	dir string
	dbs map[string][]string
	err error
}

// scanBackupDir walks dir one level deep for <db>/<coll>.json files.
func scanBackupDir(dir string) (map[string][]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot access %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", dir, err)
	}

	out := map[string][]string{}
	total := 0
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var colls []string
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			colls = append(colls, strings.TrimSuffix(f.Name(), ".json"))
		}
		if len(colls) > 0 {
			sort.Strings(colls)
			out[e.Name()] = colls
			total += len(colls)
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("no backups found in %q (expected <database>/<collection>.json)", dir)
	}
	return out, nil
}

func scanBackupsCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		dbs, err := scanBackupDir(dir)
		return backupScanResultMsg{dir: dir, dbs: dbs, err: err}
	}
}

// ---------------------------------------------------------------------------
// Operation progress (sent from the runner goroutine over a channel)
// ---------------------------------------------------------------------------

type itemStartMsg struct{ db, coll string }

type itemProgressMsg struct {
	db, coll string
	done     int64
	total    int64
}

type itemDoneMsg struct {
	db, coll string
	docs     int64
	err      error
}

type opDoneMsg struct {
	failed   int
	canceled bool
}

// opLaunchedMsg acknowledges that the operation goroutine/cmd started.
type opLaunchedMsg struct{}

// chClosedMsg is delivered if the progress channel closes unexpectedly.
type chClosedMsg struct{}

package ui

import (
	"context"
	"time"

	"mongodumpx/operations"

	tea "github.com/charmbracelet/bubbletea"
)

// progressTick is the minimum interval between progress messages for a
// single collection (operations may report per-document).
const progressTick = 100 * time.Millisecond

// runOperationCmd performs the whole backup/restore on a command goroutine,
// delivering progress messages over ch. The channel is closed when done.
func runOperationCmd(ctx context.Context, mode Mode, items []workItem, outDir string, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		runOperation(ctx, mode, items, outDir, ch)
		return opLaunchedMsg{}
	}
}

func runOperation(ctx context.Context, mode Mode, items []workItem, outDir string, ch chan tea.Msg) {
	defer close(ch)

	failed := 0
	for _, it := range items {
		if ctx.Err() != nil {
			ch <- opDoneMsg{failed: failed, canceled: true}
			return
		}

		ch <- itemStartMsg{db: it.db, coll: it.coll}

		report := throttledReporter(ch, it.db, it.coll)

		var docs int64
		var err error
		switch mode {
		case ModeBackup:
			docs, err = operations.ExportCollection(ctx, it.db, it.coll, outDir, report)
		case ModeRestore:
			docs, err = operations.ImportCollection(ctx, it.path, it.db, it.coll, report)
		}

		if err != nil {
			if ctx.Err() != nil {
				ch <- opDoneMsg{failed: failed, canceled: true}
				return
			}
			failed++
		}
		ch <- itemDoneMsg{db: it.db, coll: it.coll, docs: docs, err: err}
	}

	ch <- opDoneMsg{failed: failed}
}

// throttledReporter limits progress messages to one per progressTick.
func throttledReporter(ch chan tea.Msg, db, coll string) func(done, total int64) {
	var last time.Time
	return func(done, total int64) {
		now := time.Now()
		if now.Sub(last) < progressTick {
			return
		}
		last = now
		ch <- itemProgressMsg{db: db, coll: coll, done: done, total: total}
	}
}

// waitMsg blocks until the next message arrives on ch.
func waitMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return chClosedMsg{}
		}
		return msg
	}
}

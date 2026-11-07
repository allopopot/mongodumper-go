package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logKind int

const (
	logOK logKind = iota
	logErr
	logInfo
)

type logLine struct {
	kind logKind
	text string
}

type progressModel struct {
	mode     Mode
	prevPage Page
	target   string // where backups go / were read from

	items        []workItem
	completed    int
	failed       int
	currentKey   string
	currentDone  int64
	currentTotal int64

	overall progress.Model
	current progress.Model
	spinner spinner.Model

	running   bool
	canceling bool
	finished  bool
	canceled  bool

	cancel context.CancelFunc
	ch     chan tea.Msg

	logs   []logLine
	width  int
	height int
}

func newProgressModel(mode Mode, items []workItem, target string, prev Page, cancel context.CancelFunc, ch chan tea.Msg) progressModel {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return progressModel{
		mode:     mode,
		prevPage: prev,
		target:   target,
		items:    items,
		cancel:   cancel,
		ch:       ch,
		overall:  progress.New(progress.WithDefaultGradient(), progress.WithWidth(60), progress.WithoutPercentage()),
		current:  progress.New(progress.WithDefaultGradient(), progress.WithWidth(60)),
		spinner:  sp,
		running:  true,
		width:    80,
		height:   24,
	}
}

func (m progressModel) update(msg tea.Msg) (progressModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.overall.Width = max(20, m.width-14)
		m.current.Width = max(20, m.width-4)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.running {
				if !m.canceling {
					m.canceling = true
					m.cancel()
					m.logs = append(m.logs, logLine{logInfo, "canceling…"})
				}
				return m, nil
			}
			return m, goTo(m.prevPage)
		case "enter":
			if !m.finished {
				return m, nil
			}
			return m, goTo(m.prevPage)
		}
		return m, nil

	case spinner.TickMsg:
		if !m.running {
			return m, nil // stop ticking once the operation ends
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case itemStartMsg:
		m.currentKey = msg.db + "." + msg.coll
		m.currentDone, m.currentTotal = 0, 0
		cmds = append(cmds, m.current.SetPercent(0), m.pump())
		return m, tea.Batch(cmds...)

	case itemProgressMsg:
		if m.currentKey != msg.db+"."+msg.coll {
			return m, m.pump()
		}
		m.currentDone, m.currentTotal = msg.done, msg.total
		pct := 0.0
		if msg.total > 0 {
			pct = clamp01(float64(msg.done) / float64(msg.total))
		}
		cmds = append(cmds, m.current.SetPercent(pct), m.pump())
		return m, tea.Batch(cmds...)

	case itemDoneMsg:
		m.completed++
		if msg.err != nil {
			m.failed++
			m.logs = append(m.logs, logLine{logErr, "✘ " + msg.db + "." + msg.coll + " — " + msg.err.Error()})
		} else {
			m.logs = append(m.logs, logLine{logOK, "✔ " + msg.db + "." + msg.coll + " — " + comma(msg.docs) + " docs"})
			// The throttled reporter may have dropped the final tick, so
			// bring the amount line up to date before pinning the bar.
			if m.currentTotal > 0 {
				m.currentDone = m.currentTotal
			} else if m.mode == ModeBackup {
				m.currentDone, m.currentTotal = msg.docs, msg.docs
			}
			cmds = append(cmds, m.current.SetPercent(1))
		}
		cmds = append(cmds, m.overall.SetPercent(m.overallPct()), m.pump())
		return m, tea.Batch(cmds...)

	case opDoneMsg:
		m.running = false
		m.finished = true
		m.canceled = msg.canceled
		if msg.canceled {
			m.logs = append(m.logs, logLine{logInfo, "operation canceled"})
		}
		cmds = append(cmds, m.overall.SetPercent(m.overallPct()))
		return m, tea.Batch(cmds...)

	case chClosedMsg:
		m.running = false
		m.finished = true
		return m, nil

	default:
		// Let the progress bars animate (FrameMsg) and ignore anything else.
		var c tea.Cmd
		var model tea.Model
		model, c = m.overall.Update(msg)
		m.overall = model.(progress.Model)
		cmds = append(cmds, c)
		model, c = m.current.Update(msg)
		m.current = model.(progress.Model)
		cmds = append(cmds, c)
		return m, tea.Batch(cmds...)
	}
}

// pump re-issues the channel read while the operation is still running,
// keeping exactly one pending reader at all times.
func (m progressModel) pump() tea.Cmd {
	if m.running && m.ch != nil {
		return waitMsg(m.ch)
	}
	return nil
}

// overallPct is completed items plus the current item's fraction.
func (m progressModel) overallPct() float64 {
	if len(m.items) == 0 {
		return 1
	}
	frac := 0.0
	if m.running && m.currentTotal > 0 {
		frac = clamp01(float64(m.currentDone) / float64(m.currentTotal))
	}
	return clamp01((float64(m.completed) + frac) / float64(len(m.items)))
}

func (m progressModel) view(width, height int) string {
	name := "Backup"
	if m.mode == ModeRestore {
		name = "Restore"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(name + " — " + m.target))
	b.WriteString("\n\n")

	// Status line.
	switch {
	case m.canceling:
		b.WriteString("  " + m.spinner.View() + " " + warnStyle.Render("canceling…") + "\n")
	case m.running:
		b.WriteString("  " + m.spinner.View() + " " + infoStyle.Render("running… (esc to cancel)") + "\n")
	case m.canceled:
		ok := m.completed - m.failed
		b.WriteString("  " + warnStyle.Render("Canceled — "+itoa(ok)+" of "+itoa(len(m.items))+" collections completed") + "\n")
	case m.failed > 0:
		b.WriteString("  " + errStyle.Render("Finished with errors — "+itoa(m.completed-m.failed)+" ok, "+itoa(m.failed)+" failed") + "\n")
	default:
		b.WriteString("  " + okStyle.Render("Done — "+itoa(m.completed)+" collections") + "\n")
	}

	// Overall progress.
	b.WriteString("\n  Overall  " + m.overall.View() + "  " +
		infoStyle.Render(itoa(m.completed)+"/"+itoa(len(m.items))+" collections") + "\n")

	// Current collection progress.
	if m.currentKey != "" {
		b.WriteString("\n  " + m.currentKey + "\n")
		b.WriteString("  " + m.current.View() + "\n")
		amount := formatAmount(m.mode, m.currentDone)
		if m.currentTotal > 0 {
			amount += " / " + formatAmount(m.mode, m.currentTotal)
		}
		b.WriteString("  " + infoStyle.Render(amount) + "\n")
	}

	// Result log.
	maxLines := clampInt(height-15, 2, 30)
	b.WriteString("\n  " + paneTitleStyle.Render("Log") + "\n")
	start := clampInt(len(m.logs)-maxLines, 0, max(0, len(m.logs)-1))
	if len(m.logs) == 0 {
		start = 0
	}
	for _, l := range m.logs[start:] {
		switch l.kind {
		case logOK:
			b.WriteString("  " + okStyle.Render(l.text) + "\n")
		case logErr:
			b.WriteString("  " + errStyle.Render(l.text) + "\n")
		default:
			b.WriteString("  " + infoStyle.Render(l.text) + "\n")
		}
	}

	// Hints.
	b.WriteString("\n")
	if m.finished {
		b.WriteString(hintStyle.Render("  enter/esc: back · q: quit"))
	} else {
		b.WriteString(hintStyle.Render("  esc: cancel · ctrl+c: quit"))
	}
	return b.String()
}

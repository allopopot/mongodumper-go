package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type restoreLocModel struct {
	input    textinput.Model
	err      string
	scanning bool
	spinner  spinner.Model
}

func newRestoreLocModel() restoreLocModel {
	ti := textinput.New()
	ti.Prompt = "Path> "
	ti.Placeholder = defaultBackupDir
	ti.CharLimit = 512
	ti.Width = 60

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return restoreLocModel{input: ti, spinner: sp}
}

func (m restoreLocModel) update(msg tea.Msg) (restoreLocModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.scanning {
			return m, nil
		}
		if msg.String() == "enter" {
			dir := strings.TrimSpace(m.input.Value())
			if dir == "" {
				dir = defaultBackupDir
			}
			m.err = ""
			m.scanning = true
			return m, tea.Batch(scanBackupsCmd(dir), m.spinner.Tick)
		}
		if msg.String() == "esc" {
			return m, goTo(pageMode)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		if !m.scanning {
			return m, nil // stop ticking
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	// Forward everything else (cursor blink ticks) to the input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m restoreLocModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Restore — backup location"))
	b.WriteString("\n\n")
	b.WriteString("  Directory containing <database>/<collection>.json files\n\n")
	b.WriteString("  " + m.input.View() + "\n")

	switch {
	case m.scanning:
		b.WriteString("  " + m.spinner.View() + " scanning…\n")
	case m.err != "":
		b.WriteString("  " + errStyle.Render("✘ "+m.err) + "\n")
	default:
		b.WriteString("\n")
	}

	b.WriteString("\n" + hintStyle.Render("  enter: continue · esc: back · ctrl+c: quit"))
	return b.String()
}

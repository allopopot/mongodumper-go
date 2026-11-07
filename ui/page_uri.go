package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultURI = "mongodb://localhost:27017"

type uriModel struct {
	input      textinput.Model
	err        string
	connecting bool
	spinner    spinner.Model
}

func newURIModel() uriModel {
	ti := textinput.New()
	ti.Prompt = "URI> "
	ti.Placeholder = defaultURI
	ti.CharLimit = 512
	ti.Width = 60

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := uriModel{input: ti, spinner: sp}
	// Focus must be part of the initial state: Model.Init has a value
	// receiver, so Focus() called there would only mutate a copy.
	m.input.Focus()
	return m
}

func (m uriModel) update(msg tea.Msg) (uriModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.connecting {
			return m, nil
		}
		if msg.String() == "enter" {
			uri := strings.TrimSpace(m.input.Value())
			if uri == "" {
				uri = defaultURI
			}
			m.err = ""
			m.connecting = true
			return m, tea.Batch(connectCmd(uri), m.spinner.Tick)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		if !m.connecting {
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

func (m uriModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MongoDB Backup & Restore"))
	b.WriteString("\n\n")
	b.WriteString("  Connect to a MongoDB server\n\n")
	b.WriteString("  " + m.input.View() + "\n")

	switch {
	case m.connecting:
		b.WriteString("  " + m.spinner.View() + " connecting…\n")
	case m.err != "":
		b.WriteString("  " + errStyle.Render("✘ "+m.err) + "\n")
	default:
		b.WriteString("\n")
	}

	b.WriteString("\n" + hintStyle.Render("  enter: connect · esc/ctrl+c: quit"))
	return b.String()
}

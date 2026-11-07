package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type modeModel struct {
	cursor int // 0 = backup, 1 = restore
}

func (m modeModel) update(msg tea.Msg) (modeModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "up", "left":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "right":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		mode := ModeBackup
		if m.cursor == 1 {
			mode = ModeRestore
		}
		return m, func() tea.Msg { return modeChosenMsg{mode: mode} }
	case "esc":
		return m, goTo(pageURI)
	}
	return m, nil
}

func (m modeModel) view() string {
	options := []struct {
		name string
		desc string
	}{
		{"Backup", "dump databases to " + defaultBackupDir},
		{"Restore", "load a backup from disk"},
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose operation"))
	b.WriteString("\n\n")

	for i, opt := range options {
		prefix := "    "
		line := padRight(opt.name, 10) + infoStyle.Render(opt.desc)
		if i == m.cursor {
			prefix = "  ▸ "
			line = cursorStyle.Render(padRight(opt.name, 10)) + opt.desc
		}
		b.WriteString("  " + prefix + line + "\n")
	}

	b.WriteString("\n" + hintStyle.Render("  ↑/↓: choose · enter: confirm · esc: back · q: quit"))
	return b.String()
}

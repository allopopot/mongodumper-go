package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type InitModel struct {
	viewState string
	opsMode   string
}

func (m InitModel) Init() tea.Cmd {
	return tea.Batch(ConnectionForm.Init())
}

func (m InitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	_, cmd := ConnectionUpdate(m, msg)

	return m, cmd
}

func (m InitModel) View() tea.View {
	s := "// MongoDumper //"
	s += strings.Repeat("/", 500)
	s += "\n\n"

	switch m.viewState {
	case "connection":
		s += ConnectionView(m)
	}

	s += "\nPress q to quit.\n"

	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

func InitializeModel() InitModel {
	return InitModel{
		viewState: "connection",
	}
}

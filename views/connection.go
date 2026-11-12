package views

import (
	"mongodumpx/dbcontext"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

var ConnectionForm = huh.NewForm(
	huh.NewGroup(
		huh.NewInput().
			Title("MongoDB URI").
			Placeholder("Please enter your MongoDB URI connection string...").
			Validate(func(s string) error {
				_, err := dbcontext.InitDB(s)
				if err != nil {
					return err
				}
				return nil
			}),
		huh.NewSelect[string]().Title("Operation Mode").Options(
			huh.NewOption("Backup", "backup"),
			huh.NewOption("Restore", "restore"),
		).Key("ops_mode"),
	).Title("Connection"),
)

func ConnectionUpdate(m InitModel, msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := ConnectionForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		ConnectionForm = f
	}

	return m, cmd
}

func ConnectionView(m InitModel) string {
	if ConnectionForm.State == huh.StateCompleted {
		m.opsMode = ConnectionForm.GetString("ops_mode")
		return m.opsMode
	} else {
		return ConnectionForm.View()
	}
}

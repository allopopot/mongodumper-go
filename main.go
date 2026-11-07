package main

import (
	"context"
	"fmt"
	"os"

	"mongodumpx/dbcontext"
	"mongodumpx/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = dbcontext.Disconnect(context.Background())
}

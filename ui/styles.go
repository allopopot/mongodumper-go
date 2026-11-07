package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	paneFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("205")).
				Padding(0, 1)
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
	paneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	dimCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	checkedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	uncheckedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// comma formats n with thousands separators.
func comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// formatBytes renders a byte count in human-readable form.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return strconv.FormatFloat(float64(b)/(1<<30), 'f', 2, 64) + " GiB"
	case b >= 1<<20:
		return strconv.FormatFloat(float64(b)/(1<<20), 'f', 1, 64) + " MiB"
	case b >= 1<<10:
		return strconv.FormatFloat(float64(b)/(1<<10), 'f', 1, 64) + " KiB"
	default:
		return strconv.FormatInt(b, 10) + " B"
	}
}

// formatAmount renders a progress amount according to the operation mode.
func formatAmount(mode Mode, n int64) string {
	if mode == ModeBackup {
		return comma(n) + " docs"
	}
	return formatBytes(n)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// padRight pads s with spaces to width w (by display width).
func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

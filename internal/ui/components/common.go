package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/sysatom/lnd/internal/ui"
)

func Header(title string, version string) string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		ui.TitleStyle.Render(title),
		ui.SubtitleStyle.Render(version),
	)
}

func Footer(msg string) string {
	return ui.SubtleStyle.Render(msg)
}

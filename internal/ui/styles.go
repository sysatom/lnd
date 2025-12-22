package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#04B575")
	ErrorColor     = lipgloss.Color("#FF0000")
	WarningColor   = lipgloss.Color("#FFA500")
	SubtleColor    = lipgloss.Color("#626262")

	// Text Styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(SubtleColor)

	// Layout Styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SubtleColor).
			Padding(0, 1)

	TabStyle = lipgloss.NewStyle().
			Border(lipgloss.HiddenBorder()).
			Padding(0, 1)

	ActiveTabStyle = TabStyle.Copy().
			Border(lipgloss.NormalBorder()).
			BorderForeground(PrimaryColor).
			Foreground(PrimaryColor).
			Bold(true)
)

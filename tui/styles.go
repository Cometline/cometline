package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			MarginBottom(1)

	reasoningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			MarginBottom(1)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			MarginBottom(1)

	toolStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Foreground(lipgloss.Color("214")).
			Padding(0, 1).
			MarginBottom(1)

	toolErrStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Foreground(lipgloss.Color("214")).
			Padding(0, 1).
			MarginBottom(1)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginBottom(1)

	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

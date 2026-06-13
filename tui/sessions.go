package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

type sessItem struct {
	id, title, model string
}

func (s sessItem) Title() string {
	t := strings.TrimSpace(s.title)
	if t == "" {
		return "(untitled)"
	}
	return t
}

func (s sessItem) Description() string {
	return s.id + " · " + s.model
}

func (s sessItem) FilterValue() string {
	return s.Title() + " " + s.Description()
}

func newSessionDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(lipgloss.Color("214")).Bold(true)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(lipgloss.Color("244"))
	return d
}

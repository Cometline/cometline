package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cometline/cometmind/internal/session"
)

type screenKind int

const (
	screenList screenKind = iota
	screenChat
)

// AppModel is the Bubble Tea root: session picker vs chat.
type AppModel struct {
	deps *Deps

	screen screenKind
	slist  list.Model
	chat   *chatModel

	// Last terminal size (Bubble Tea sends WindowSizeMsg once; chat is created later and needs these).
	winW int
	winH int
}

// NewApp builds the root model; caller must call SetProgram before Run.
func NewApp(d *Deps) *AppModel {
	return &AppModel{deps: d}
}

// SetProgram wires async agent streaming back into the program loop.
func (m *AppModel) SetProgram(p *tea.Program) {
	m.deps.Program = p
}

// Init implements tea.Model.
func (m *AppModel) Init() tea.Cmd {
	m.rebuildSessionList()
	return nil
}

func (m *AppModel) rebuildSessionList() {
	ctx := context.Background()
	rows, err := m.deps.Sessions.ListSessions(ctx, m.deps.WorkspaceID)
	items := make([]list.Item, 0)
	if err == nil {
		items = make([]list.Item, 0, len(rows))
		for _, s := range rows {
			items = append(items, sessItem{id: s.ID, title: s.Title, model: s.ModelID})
		}
	}
	del := newSessionDelegate()
	w, h := 80, 24
	if m.slist.Width() != 0 {
		w = m.slist.Width()
	}
	if m.slist.Height() != 0 {
		h = m.slist.Height()
	}
	m.slist = list.New(items, del, w, h)
	m.slist.Title = "Sessions · enter opens · n new · s list · q quit"
	m.slist.DisableQuitKeybindings()
	m.slist.SetShowHelp(false)
}

// Update implements tea.Model.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.winW = msg.Width
		m.winH = msg.Height
		m.slist.SetWidth(msg.Width)
		m.slist.SetHeight(msg.Height - 4)
		if m.screen == screenChat && m.chat != nil {
			nmod, ccmd := m.chat.Update(msg)
			m.chat = nmod.(*chatModel)
			return m, ccmd
		}
		return m, nil

	case SessionBackMsg:
		m.screen = screenList
		m.chat = nil
		m.rebuildSessionList()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		composerShortcutGate := m.screen == screenChat && m.chat != nil && m.chat.ComposerHasText()

		if !composerShortcutGate {
			switch msg.String() {
			case "s":
				if m.screen == screenChat {
					m.screen = screenList
					m.chat = nil
					m.rebuildSessionList()
					return m, nil
				}
			case "n":
				return m.openNewSession()
			}
		}

		if m.screen == screenList {
			switch msg.String() {
			case "enter":
				if it, ok := m.slist.SelectedItem().(sessItem); ok {
					return m.openSession(it.id)
				}
			}
			var cmd tea.Cmd
			m.slist, cmd = m.slist.Update(msg)
			return m, cmd
		}

		if m.chat != nil {
			nmod, cmd := m.chat.Update(msg)
			m.chat = nmod.(*chatModel)
			return m, cmd
		}
	}

	if m.screen == screenList {
		var cmd tea.Cmd
		m.slist, cmd = m.slist.Update(msg)
		return m, cmd
	}
	if m.chat != nil {
		nmod, cmd := m.chat.Update(msg)
		m.chat = nmod.(*chatModel)
		return m, cmd
	}
	return m, nil
}

func (m *AppModel) openNewSession() (tea.Model, tea.Cmd) {
	ctx := context.Background()
	sess, err := m.deps.Sessions.NewSession(ctx, m.deps.WorkspaceID, m.deps.Config.Model, m.deps.Config.Provider)
	if err != nil {
		m.rebuildSessionList()
		return m, nil
	}
	turn := session.AgentTurnFromSession(sess)
	ch := newChatModel(m.deps, turn, sess.Title)
	if err := ch.loadTranscript(ctx); err != nil {
		ch.lines = append(ch.lines, chatLine{kind: lineErr, text: err.Error()})
		ch.refreshVP()
	}
	m.chat = ch
	m.screen = screenChat
	m.applyChatLayout(ch)
	return m, m.chat.Init()
}

func (m *AppModel) openSession(id string) (tea.Model, tea.Cmd) {
	ctx := context.Background()
	sess, err := m.deps.Sessions.GetSession(ctx, id)
	if err != nil {
		m.rebuildSessionList()
		return m, nil
	}
	turn := session.AgentTurnFromSession(sess)
	ch := newChatModel(m.deps, turn, sess.Title)
	if err := ch.loadTranscript(ctx); err != nil {
		ch.lines = append(ch.lines, chatLine{kind: lineErr, text: err.Error()})
		ch.refreshVP()
	}
	m.chat = ch
	m.screen = screenChat
	m.applyChatLayout(ch)
	return m, m.chat.Init()
}

// applyChatLayout sizes the transcript viewport using the last WindowSizeMsg (chat never saw that msg while we were on the list).
func (m *AppModel) applyChatLayout(ch *chatModel) {
	w, h := m.winW, m.winH
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	ch.layout(w, h)
}

// View implements tea.Model.
func (m *AppModel) View() string {
	switch m.screen {
	case screenList:
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.slist.View(),
			helpStyle.Render("enter open · n new · s returns from chat when composer empty · q quit"),
		)
	case screenChat:
		if m.chat != nil {
			return m.chat.View()
		}
	}
	return ""
}

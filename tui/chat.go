package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

type chatLineKind int

const (
	lineUser chatLineKind = iota
	lineAssistant
	lineReasoning
	lineTool
	lineErr
	lineMeta
)

type chatLine struct {
	kind      chatLineKind
	text      string
	toolName  string
	toolIn    string
	toolOut   string
	toolErr   bool
	streaming bool
}

type chatModel struct {
	deps *Deps

	turn    session.AgentTurn
	title   string
	session string

	vp viewport.Model
	ta textarea.Model

	lines   []chatLine
	footer  string
	running bool

	cancelRun context.CancelFunc
}

func newChatModel(d *Deps, turn session.AgentTurn, title string) *chatModel {
	ta := textarea.New()
	ta.Placeholder = "Message… (shift+enter newline)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("shift+enter", "ctrl+enter"))
	ta.SetHeight(4)
	ta.Focus()

	return &chatModel{
		deps:    d,
		turn:    turn,
		title:   title,
		session: turn.ID,
		ta:      ta,
		vp:      viewport.New(0, 0),
	}
}

// ComposerHasText is used by AppModel to avoid stealing s/n keys while typing.
func (m *chatModel) ComposerHasText() bool {
	return strings.TrimSpace(m.ta.Value()) != ""
}

func (m *chatModel) loadTranscript(ctx context.Context) error {
	entries, err := m.deps.Sessions.LoadTranscript(ctx, m.turn.ID)
	if err != nil {
		return err
	}
	m.lines = m.lines[:0]
	for _, e := range entries {
		switch e.Kind {
		case session.TranscriptKindUser:
			m.lines = append(m.lines, chatLine{kind: lineUser, text: e.Text})
		case session.TranscriptKindReasoning:
			m.lines = append(m.lines, chatLine{kind: lineReasoning, text: e.Text})
		case session.TranscriptKindAssistant:
			m.lines = append(m.lines, chatLine{kind: lineAssistant, text: e.Text})
		case session.TranscriptKindTool:
			m.lines = append(m.lines, chatLine{
				kind:     lineTool,
				toolName: e.ToolName,
				toolIn:   e.ToolInput,
				toolOut:  e.ToolOutput,
				toolErr:  e.ToolIsError,
			})
		}
	}
	m.refreshVP()
	return nil
}

func (m *chatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *chatModel) refreshVP() {
	m.vp.SetContent(m.renderTranscript())
	m.vp.GotoBottom()
}

func trimToolJSON(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (m *chatModel) renderTranscript() string {
	var b strings.Builder
	for _, ln := range m.lines {
		switch ln.kind {
		case lineUser:
			b.WriteString(userStyle.Render("You"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Width(m.vp.Width).Render(ln.text))
			b.WriteString("\n\n")
		case lineReasoning:
			st := reasoningStyle
			if ln.streaming {
				st = st.Copy().Faint(true)
			}
			b.WriteString(st.Render("Reasoning"))
			b.WriteString("\n")
			b.WriteString(reasoningStyle.Width(m.vp.Width).Render(ln.text))
			b.WriteString("\n\n")
		case lineAssistant:
			st := assistantStyle
			if ln.streaming {
				st = st.Copy().Faint(true)
			}
			b.WriteString(st.Render("Assistant"))
			b.WriteString("\n")
			b.WriteString(assistantStyle.Width(m.vp.Width).Render(ln.text))
			b.WriteString("\n\n")
		case lineTool:
			st := toolStyle
			if ln.toolErr {
				st = toolErrStyle
			}
			head := fmt.Sprintf("Tool · %s", ln.toolName)
			body := trimToolJSON(ln.toolIn, 800)
			var out string
			if ln.toolOut != "" {
				out = "\n──\n" + trimToolJSON(ln.toolOut, 1200)
			}
			b.WriteString(st.Render(head + "\n" + body + out))
			b.WriteString("\n\n")
		case lineErr:
			b.WriteString(errStyle.Render(ln.text))
			b.WriteString("\n\n")
		case lineMeta:
			b.WriteString(metaStyle.Render(ln.text))
			b.WriteString("\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *chatModel) layout(w, h int) {
	header := 3
	footer := 3
	help := 1
	taH := m.ta.Height()
	vph := h - header - taH - footer - help
	if vph < 6 {
		vph = 6
	}
	m.vp.Width = w - 2
	m.vp.Height = vph
	m.ta.SetWidth(w - 4)
	m.refreshVP()
}

func (m *chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.running && m.cancelRun != nil {
				m.cancelRun()
				return m, nil
			}
			return m, func() tea.Msg { return SessionBackMsg{} }
		}
		ks := msg.String()
		if (msg.Type == tea.KeyEnter && !strings.Contains(ks, "shift")) || ks == "enter" {
			if txt := strings.TrimSpace(m.ta.Value()); txt != "" && !m.running {
				return m.submit()
			}
		}

	case AgentEventMsg:
		m.applyEvent(msg.Event)
		m.refreshVP()

	case RunFinishedMsg:
		m.running = false
		m.cancelRun = nil
		m.finalizeStreaming()
		m.refreshVP()
		if msg.Err != nil && !errors.Is(msg.Err, context.Canceled) {
			m.lines = append(m.lines, chatLine{kind: lineErr, text: "run: " + msg.Err.Error()})
			m.refreshVP()
		}
	}

	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
}

func (m *chatModel) finalizeStreaming() {
	for i := range m.lines {
		m.lines[i].streaming = false
	}
}

func (m *chatModel) finalizeStreamingReasoning() {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].kind == lineReasoning && m.lines[i].streaming {
			m.lines[i].streaming = false
			return
		}
	}
}

func (m *chatModel) finalizeStreamingAssistant() {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].kind == lineAssistant && m.lines[i].streaming {
			m.lines[i].streaming = false
			return
		}
	}
}

func (m *chatModel) appendReasoning(delta string) {
	if len(m.lines) > 0 && m.lines[len(m.lines)-1].kind == lineReasoning && m.lines[len(m.lines)-1].streaming {
		m.lines[len(m.lines)-1].text += delta
		return
	}
	m.lines = append(m.lines, chatLine{kind: lineReasoning, text: delta, streaming: true})
}

func (m *chatModel) appendAssistant(delta string) {
	m.finalizeStreamingReasoning()
	if len(m.lines) > 0 && m.lines[len(m.lines)-1].kind == lineAssistant && m.lines[len(m.lines)-1].streaming {
		m.lines[len(m.lines)-1].text += delta
		return
	}
	m.lines = append(m.lines, chatLine{kind: lineAssistant, text: delta, streaming: true})
}

func (m *chatModel) applyEvent(ev event.Event) {
	switch ev.Kind {
	case event.KindReasoningStart:
		m.finalizeStreamingAssistant()
		m.appendReasoning("")
	case event.KindReasoningDelta:
		if ev.ReasoningDelta != nil {
			m.appendReasoning(ev.ReasoningDelta.Text)
		}
	case event.KindTextDelta:
		if ev.TextDelta != nil {
			m.appendAssistant(ev.TextDelta.Delta)
		}
	case event.KindToolCall:
		if ev.ToolCall != nil {
			m.finalizeStreamingReasoning()
			m.finalizeStreamingAssistant()
			m.lines = append(m.lines, chatLine{
				kind:     lineTool,
				toolName: ev.ToolCall.Tool,
				toolIn:   string(ev.ToolCall.Input),
			})
		}
	case event.KindToolResult:
		if ev.ToolOut != nil {
			for i := len(m.lines) - 1; i >= 0; i-- {
				if m.lines[i].kind == lineTool && m.lines[i].toolName == ev.ToolOut.Tool && m.lines[i].toolOut == "" {
					out := strings.TrimSpace(ev.ToolOut.Output)
					if len(out) > 1200 {
						out = out[:1200] + "…"
					}
					m.lines[i].toolOut = out
					m.lines[i].toolErr = ev.ToolOut.Err != ""
					break
				}
			}
		}
	case event.KindStepFinish:
		if ev.Step != nil {
			u := ev.Step.Usage
			m.footer = fmt.Sprintf("tokens in=%d out=%d", u.InputTokens, u.OutputTokens)
			m.lines = append(m.lines, chatLine{
				kind: lineMeta,
				text: m.footer,
			})
		}
	case event.KindError:
		if ev.Err != nil {
			m.lines = append(m.lines, chatLine{
				kind: lineErr,
				text: fmt.Sprintf("%s (%s)", ev.Err.Message, ev.Err.Code),
			})
		}
	case event.KindDone:
		m.finalizeStreaming()
	}
}

func (m *chatModel) submit() (tea.Model, tea.Cmd) {
	txt := strings.TrimSpace(m.ta.Value())
	if txt == "" || m.running {
		return m, nil
	}

	ctx := context.Background()
	if _, err := m.deps.Sessions.AppendUserMessage(ctx, m.turn.ID, txt); err != nil {
		m.lines = append(m.lines, chatLine{kind: lineErr, text: err.Error()})
		m.refreshVP()
		return m, nil
	}
	title := txt
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	if err := m.deps.Sessions.SetTitleIfEmpty(ctx, m.turn.ID, title); err != nil {
		m.lines = append(m.lines, chatLine{kind: lineErr, text: err.Error()})
		m.refreshVP()
		return m, nil
	}

	m.lines = append(m.lines, chatLine{kind: lineUser, text: txt})
	m.ta.SetValue("")
	m.ta.Blur()
	m.ta.Focus()
	m.refreshVP()

	runCtx, cancel := context.WithCancel(context.Background())
	m.cancelRun = cancel
	m.running = true

	cfg := *m.deps.Config
	cfg.Model = m.turn.ModelID
	cfg.Provider = m.turn.ProviderID
	p, err := provider.New(&cfg)
	if err != nil {
		m.running = false
		cancel()
		m.lines = append(m.lines, chatLine{kind: lineErr, text: err.Error()})
		m.refreshVP()
		return m, nil
	}
	reg := tools.NewRegistry(m.deps.WorkspacePath)
	runner := agent.Runner{
		Provider:      p,
		Sessions:      m.deps.Sessions,
		Registry:      reg,
		WorkspaceRoot: m.deps.WorkspacePath,
		MaxSteps:      cfg.MaxSteps,
		MaxTokens:     cfg.MaxTokens,
	}

	go m.bridgeRun(runCtx, &runner)

	return m, textarea.Blink
}

func (m *chatModel) bridgeRun(runCtx context.Context, runner *agent.Runner) {
	evCh := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(runCtx, m.turn, evCh)
		close(evCh)
	}()
	for ev := range evCh {
		if m.deps.Program != nil {
			m.deps.Program.Send(AgentEventMsg{Event: ev})
		}
	}
	err := <-errCh
	if m.deps.Program != nil {
		m.deps.Program.Send(RunFinishedMsg{Err: err})
	}
}

func (m *chatModel) View() string {
	head := titleStyle.Render(fmt.Sprintf("%s  ·  %s", m.titleOrUntitled(), shortID(m.session)))
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		Width(m.vp.Width + 2).
		Render(m.vp.View())
	composer := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(m.vp.Width + 2).
		Render(m.ta.View())
	status := m.footer
	if m.running {
		status = "running… (esc cancels)"
	}
	foot := helpStyle.Render(status + "   ·   esc back/cancel · enter send · shift+enter newline")
	return lipgloss.JoinVertical(lipgloss.Left, head, box, composer, foot)
}

func (m *chatModel) titleOrUntitled() string {
	t := strings.TrimSpace(m.title)
	if t == "" {
		return "(untitled)"
	}
	return t
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "…"
}

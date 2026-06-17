package feedback

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type feedbackKind int

const (
	kindFeature feedbackKind = iota
	kindBug
)

type formStep int

const (
	stepKind formStep = iota
	stepTitle
	stepBody
	stepConfirm
)

type feedbackResult struct {
	kind  feedbackKind
	title string
	body  string
}

var kindOptions = []struct {
	icon  string
	label string
}{
	{"✨", "機能リクエスト (Feature Request)"},
	{"🐛", "バグ報告 (Bug Report)"},
}

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

type formModel struct {
	step         formStep
	kindCursor   int
	titleInput   textinput.Model
	bodyInput    textarea.Model
	titleWarning bool
	cancelled    bool
	result       *feedbackResult
}

func newFormModel() formModel {
	ti := textinput.New()
	ti.Placeholder = "タイトルを入力..."
	ti.CharLimit = 256

	ta := textarea.New()
	ta.Placeholder = "詳細説明を入力... (省略可)"
	ta.SetWidth(60)
	ta.SetHeight(8)

	return formModel{
		step:       stepKind,
		titleInput: ti,
		bodyInput:  ta,
	}
}

func (m formModel) Init() tea.Cmd { return nil }

func (m formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w := msg.Width - 4
		if w < 20 {
			w = 20
		}
		if w > 76 {
			w = 76
		}
		m.bodyInput.SetWidth(w)
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.cancelled = true
			return m, tea.Quit
		}
		switch m.step {
		case stepKind:
			return m.updateKind(msg)
		case stepTitle:
			return m.updateTitle(msg)
		case stepBody:
			return m.updateBody(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m formModel) updateKind(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyUp || msg.String() == "k":
		if m.kindCursor > 0 {
			m.kindCursor--
		}
	case msg.Type == tea.KeyDown || msg.String() == "j":
		if m.kindCursor < len(kindOptions)-1 {
			m.kindCursor++
		}
	case msg.Type == tea.KeyEnter:
		m.step = stepTitle
		return m, m.titleInput.Focus()
	case msg.Type == tea.KeyEsc:
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m formModel) updateTitle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if strings.TrimSpace(m.titleInput.Value()) == "" {
			m.titleWarning = true
			return m, nil
		}
		m.titleWarning = false
		m.titleInput.Blur()
		m.step = stepBody
		return m, m.bodyInput.Focus()
	case tea.KeyEsc:
		m.titleInput.Blur()
		m.step = stepKind
		return m, nil
	}
	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

func (m formModel) updateBody(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.bodyInput.Blur()
		m.step = stepConfirm
		return m, nil
	case tea.KeyEsc:
		m.bodyInput.Blur()
		m.step = stepTitle
		return m, m.titleInput.Focus()
	}
	var cmd tea.Cmd
	m.bodyInput, cmd = m.bodyInput.Update(msg)
	return m, cmd
}

func (m formModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.result = &feedbackResult{
			kind:  feedbackKind(m.kindCursor),
			title: strings.TrimSpace(m.titleInput.Value()),
			body:  strings.TrimSpace(m.bodyInput.Value()),
		}
		return m, tea.Quit
	case "n", "N":
		m.cancelled = true
		return m, tea.Quit
	case "b", "B":
		m.step = stepBody
		return m, m.bodyInput.Focus()
	}
	if msg.Type == tea.KeyEsc {
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m formModel) View() string {
	if m.cancelled {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	switch m.step {
	case stepKind:
		b.WriteString(headerStyle.Render("gh wheel feedback") + "\n\n")
		b.WriteString("リクエストの種類を選んでください:\n\n")
		for i, opt := range kindOptions {
			line := fmt.Sprintf("%s %s", opt.icon, opt.label)
			if i == m.kindCursor {
				b.WriteString(selectedStyle.Render("▶ "+line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("↑/↓ で移動、Enter で確定、Esc でキャンセル"))

	case stepTitle:
		opt := kindOptions[m.kindCursor]
		b.WriteString(headerStyle.Render(opt.icon+" "+opt.label) + "\n\n")
		b.WriteString("タイトル:\n")
		b.WriteString(m.titleInput.View() + "\n\n")
		if m.titleWarning {
			b.WriteString(warnStyle.Render("※ タイトルは必須です") + "\n\n")
		}
		b.WriteString(dimStyle.Render("Enter で次へ、Esc で戻る"))

	case stepBody:
		opt := kindOptions[m.kindCursor]
		b.WriteString(headerStyle.Render(opt.icon+" "+opt.label) + "\n\n")
		b.WriteString("詳細説明 (省略可):\n")
		b.WriteString(m.bodyInput.View() + "\n\n")
		b.WriteString(dimStyle.Render("Tab で次へ、Esc で戻る"))

	case stepConfirm:
		opt := kindOptions[m.kindCursor]
		title := strings.TrimSpace(m.titleInput.Value())
		body := strings.TrimSpace(m.bodyInput.Value())

		b.WriteString(headerStyle.Render("プレビュー") + "\n\n")
		b.WriteString(fmt.Sprintf("種別:     %s %s\n", opt.icon, opt.label))
		b.WriteString(fmt.Sprintf("タイトル: %s\n", title))
		if body != "" {
			b.WriteString("\n本文:\n")
			b.WriteString(dimStyle.Render(body) + "\n")
		}
		b.WriteString("\n")
		b.WriteString("gh-wheel に Issue を起票しますか？\n\n")
		b.WriteString(selectedStyle.Render("[y]") + " 送信  " +
			dimStyle.Render("[n]") + " キャンセル  " +
			dimStyle.Render("[b]") + " 戻る")
	}

	b.WriteString("\n")
	return b.String()
}

// runFeedbackTUI starts the interactive form and returns the completed result.
// Returns nil if the user cancelled.
func runFeedbackTUI() (*feedbackResult, error) {
	p := tea.NewProgram(newFormModel(), tea.WithAltScreen())
	fm, err := p.Run()
	if err != nil {
		return nil, err
	}
	m, ok := fm.(formModel)
	if !ok || m.cancelled {
		return nil, nil
	}
	return m.result, nil
}

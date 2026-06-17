package task

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiTitleStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	tuiSelectStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
)

// tuiItem represents a single PR or Issue row in the TUI list.
type tuiItem struct {
	number      int
	title       string
	url         string
	state       string
	isDraft     bool
	role        string // "author", "review-requested", "both", ""
	isIssue     bool
	humanReview string // populated with --with-reviews
	aiReview    string // populated with --with-reviews
}

func (i tuiItem) FilterValue() string { return i.title }

// roleIcon returns the emoji icon for a PR role, padded to a consistent display width.
// ✏️ and 👁 are each 2 terminal columns wide, so "both" = 4 cols.
func roleIcon(role string) string {
	switch role {
	case "both":
		return "✏️👁"
	case "author":
		return "✏️  " // 2 cols emoji + 2 spaces = 4 cols
	case "review-requested":
		return " 👁 " // 1 space + 2 cols emoji + 1 space = 4 cols
	default:
		return "    " // 4 spaces for Issues
	}
}

func stateLabel(state string, isDraft bool) string {
	if isDraft {
		return "draft "
	}
	switch state {
	case "open":
		return "open  "
	case "closed":
		return "closed"
	default:
		return fmt.Sprintf("%-6s", state)
	}
}

// itemDelegate renders each row in the TUI list.
type itemDelegate struct {
	withReviews bool
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(tuiItem)
	if !ok {
		return
	}

	numStr := fmt.Sprintf("#%-4d", i.number)
	stateStr := stateLabel(i.state, i.isDraft)

	var line string
	if d.withReviews && !i.isIssue {
		line = fmt.Sprintf("  %s %s  %s  %-4s  %-6s  %s",
			roleIcon(i.role), numStr, stateStr, i.humanReview, i.aiReview, i.title)
	} else {
		line = fmt.Sprintf("  %s %s  %s  %s",
			roleIcon(i.role), numStr, stateStr, i.title)
	}

	if index == m.Index() {
		fmt.Fprint(w, tuiSelectStyle.Render(line))
	} else {
		fmt.Fprint(w, line)
	}
}

// tuiModel is the bubbletea application model for the task TUI.
type tuiModel struct {
	list   list.Model
	chosen string // URL of the selected item; empty if quit without selection
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		// Let the list handle keys while the filter input is active.
		if m.list.FilterState() == list.Filtering {
			break
		}
		if msg.String() == "enter" {
			if item, ok := m.list.SelectedItem().(tuiItem); ok {
				m.chosen = item.url
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string { return m.list.View() }

// runTUI builds the list and starts the interactive Bubble Tea session.
// After the user selects an item with Enter, opens its URL in the browser.
func runTUI(result *TaskResult, withReviews bool) error {
	items := buildTUIItems(result, withReviews)

	l := list.New(items, itemDelegate{withReviews: withReviews}, 0, 0)
	l.Title = fmt.Sprintf("%s  (%d items)", result.Repository, len(items))
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = tuiTitleStyle

	p := tea.NewProgram(tuiModel{list: l}, tea.WithAltScreen())
	fm, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := fm.(tuiModel); ok && m.chosen != "" {
		return openURL(m.chosen)
	}
	return nil
}

func buildTUIItems(result *TaskResult, withReviews bool) []list.Item {
	items := make([]list.Item, 0, len(result.PRs)+len(result.Issues))

	for _, pr := range result.PRs {
		isAuth, isRev := false, false
		for _, c := range pr.Categories {
			switch c {
			case "author":
				isAuth = true
			case "review-requested":
				isRev = true
			}
		}
		role := "author"
		switch {
		case isAuth && isRev:
			role = "both"
		case isRev:
			role = "review-requested"
		}

		it := tuiItem{
			number:  pr.Number,
			title:   pr.Title,
			url:     pr.URL,
			state:   pr.State,
			isDraft: pr.IsDraft,
			role:    role,
		}
		if withReviews {
			rs := computeReviewStates(pr.Reviews, pr.LatestCommitAt)
			it.humanReview = rs.human
			it.aiReview = rs.ai
		}
		items = append(items, it)
	}

	for _, iss := range result.Issues {
		items = append(items, tuiItem{
			number:  iss.Number,
			title:   iss.Title,
			url:     iss.URL,
			state:   iss.State,
			isIssue: true,
		})
	}

	return items
}

// openURL opens the given URL in the default system browser.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

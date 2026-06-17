package feedback

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func keyMsg(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func specialKey(t tea.KeyType) tea.Msg {
	return tea.KeyMsg{Type: t}
}

func sendKey(m formModel, msg tea.Msg) formModel {
	next, _ := m.Update(msg)
	return next.(formModel)
}

// ─── Initial state ────────────────────────────────────────────────────────────

func TestFormModel_InitialState(t *testing.T) {
	m := newFormModel()
	if m.step != stepKind {
		t.Errorf("initial step: want stepKind, got %v", m.step)
	}
	if m.kindCursor != 0 {
		t.Errorf("initial kindCursor: want 0, got %d", m.kindCursor)
	}
	if m.cancelled {
		t.Error("should not be cancelled initially")
	}
	if m.result != nil {
		t.Error("result should be nil initially")
	}
}

// ─── Kind step ────────────────────────────────────────────────────────────────

func TestFormModel_KindDownMovesCursor(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, specialKey(tea.KeyDown))
	if m.kindCursor != 1 {
		t.Errorf("after Down: want kindCursor=1, got %d", m.kindCursor)
	}
}

func TestFormModel_KindDownClampsAtEnd(t *testing.T) {
	m := newFormModel()
	m.kindCursor = len(kindOptions) - 1
	m = sendKey(m, specialKey(tea.KeyDown))
	if m.kindCursor != len(kindOptions)-1 {
		t.Errorf("cursor should not exceed last index, got %d", m.kindCursor)
	}
}

func TestFormModel_KindUpMovesCursor(t *testing.T) {
	m := newFormModel()
	m.kindCursor = 1
	m = sendKey(m, specialKey(tea.KeyUp))
	if m.kindCursor != 0 {
		t.Errorf("after Up: want kindCursor=0, got %d", m.kindCursor)
	}
}

func TestFormModel_KindUpClampsAtZero(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, specialKey(tea.KeyUp))
	if m.kindCursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", m.kindCursor)
	}
}

func TestFormModel_KindEnterAdvancesToTitle(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, specialKey(tea.KeyEnter))
	if m.step != stepTitle {
		t.Errorf("after Enter on kind: want stepTitle, got %v", m.step)
	}
}

func TestFormModel_KindEscCancels(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, specialKey(tea.KeyEsc))
	if !m.cancelled {
		t.Error("Esc on kind step should cancel")
	}
}

func TestFormModel_KindJkNavigation(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, keyMsg("j"))
	if m.kindCursor != 1 {
		t.Errorf("j: want kindCursor=1, got %d", m.kindCursor)
	}
	m = sendKey(m, keyMsg("k"))
	if m.kindCursor != 0 {
		t.Errorf("k: want kindCursor=0, got %d", m.kindCursor)
	}
}

// ─── Title step ───────────────────────────────────────────────────────────────

func advanceToTitle(t *testing.T) formModel {
	t.Helper()
	m := newFormModel()
	return sendKey(m, specialKey(tea.KeyEnter))
}

func TestFormModel_TitleEmptyBlocksAdvance(t *testing.T) {
	m := advanceToTitle(t)
	m = sendKey(m, specialKey(tea.KeyEnter))
	if m.step != stepTitle {
		t.Errorf("empty title: step should remain stepTitle, got %v", m.step)
	}
	if !m.titleWarning {
		t.Error("titleWarning should be set after failed advance")
	}
}

func TestFormModel_TitleNonEmptyAdvancesToBody(t *testing.T) {
	m := advanceToTitle(t)
	m.titleInput.SetValue("my feature title")
	m = sendKey(m, specialKey(tea.KeyEnter))
	if m.step != stepBody {
		t.Errorf("after Enter with title: want stepBody, got %v", m.step)
	}
}

func TestFormModel_TitleEscGoesBackToKind(t *testing.T) {
	m := advanceToTitle(t)
	m = sendKey(m, specialKey(tea.KeyEsc))
	if m.step != stepKind {
		t.Errorf("Esc on title: want stepKind, got %v", m.step)
	}
}

func TestFormModel_TitleWarningClearedOnAdvance(t *testing.T) {
	m := advanceToTitle(t)
	m = sendKey(m, specialKey(tea.KeyEnter)) // trigger warning
	m.titleInput.SetValue("valid title")
	m = sendKey(m, specialKey(tea.KeyEnter))
	if m.step != stepBody {
		t.Errorf("want stepBody, got %v", m.step)
	}
	if m.titleWarning {
		t.Error("titleWarning should be cleared after successful advance")
	}
}

// ─── Body step ────────────────────────────────────────────────────────────────

func advanceToBody(t *testing.T) formModel {
	t.Helper()
	m := advanceToTitle(t)
	m.titleInput.SetValue("some title")
	return sendKey(m, specialKey(tea.KeyEnter))
}

func TestFormModel_BodyTabAdvancesToConfirm(t *testing.T) {
	m := advanceToBody(t)
	m = sendKey(m, specialKey(tea.KeyTab))
	if m.step != stepConfirm {
		t.Errorf("Tab on body: want stepConfirm, got %v", m.step)
	}
}

func TestFormModel_BodyEscGoesBackToTitle(t *testing.T) {
	m := advanceToBody(t)
	m = sendKey(m, specialKey(tea.KeyEsc))
	if m.step != stepTitle {
		t.Errorf("Esc on body: want stepTitle, got %v", m.step)
	}
}

// ─── Confirm step ─────────────────────────────────────────────────────────────

func advanceToConfirm(t *testing.T) formModel {
	t.Helper()
	m := advanceToBody(t)
	return sendKey(m, specialKey(tea.KeyTab))
}

func TestFormModel_ConfirmYSubmits(t *testing.T) {
	m := advanceToConfirm(t)
	m = sendKey(m, keyMsg("y"))
	if m.result == nil {
		t.Fatal("result should be set after y")
	}
	if m.result.title != "some title" {
		t.Errorf("result.title: want %q, got %q", "some title", m.result.title)
	}
	if m.result.kind != kindFeature {
		t.Errorf("result.kind: want kindFeature, got %v", m.result.kind)
	}
}

func TestFormModel_ConfirmNCancels(t *testing.T) {
	m := advanceToConfirm(t)
	m = sendKey(m, keyMsg("n"))
	if !m.cancelled {
		t.Error("n on confirm should cancel")
	}
	if m.result != nil {
		t.Error("result should remain nil on cancel")
	}
}

func TestFormModel_ConfirmEscCancels(t *testing.T) {
	m := advanceToConfirm(t)
	m = sendKey(m, specialKey(tea.KeyEsc))
	if !m.cancelled {
		t.Error("Esc on confirm should cancel")
	}
}

func TestFormModel_ConfirmBGoesBackToBody(t *testing.T) {
	m := advanceToConfirm(t)
	m = sendKey(m, keyMsg("b"))
	if m.step != stepBody {
		t.Errorf("b on confirm: want stepBody, got %v", m.step)
	}
}

func TestFormModel_BugKindInResult(t *testing.T) {
	m := newFormModel()
	m = sendKey(m, specialKey(tea.KeyDown)) // select bug
	m = sendKey(m, specialKey(tea.KeyEnter))
	m.titleInput.SetValue("a bug")
	m = sendKey(m, specialKey(tea.KeyEnter))
	m = sendKey(m, specialKey(tea.KeyTab))
	m = sendKey(m, keyMsg("y"))
	if m.result == nil {
		t.Fatal("result should be set")
	}
	if m.result.kind != kindBug {
		t.Errorf("result.kind: want kindBug, got %v", m.result.kind)
	}
}

// ─── Ctrl+C ───────────────────────────────────────────────────────────────────

func TestFormModel_CtrlCCancelsFromAnyStep(t *testing.T) {
	models := []formModel{
		newFormModel(),
		advanceToTitle(t),
		advanceToBody(t),
		advanceToConfirm(t),
	}
	for _, m := range models {
		got := sendKey(m, specialKey(tea.KeyCtrlC))
		if !got.cancelled {
			t.Errorf("Ctrl+C should cancel at step %v", m.step)
		}
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func TestFormModel_ViewRendersEachStep(t *testing.T) {
	steps := map[string]formModel{
		"kind":    newFormModel(),
		"title":   advanceToTitle(t),
		"body":    advanceToBody(t),
		"confirm": advanceToConfirm(t),
	}
	for name, m := range steps {
		if v := m.View(); v == "" {
			t.Errorf("View() at step %q should not be empty", name)
		}
	}
}

func TestFormModel_KindViewContainsOptions(t *testing.T) {
	v := newFormModel().View()
	for _, opt := range kindOptions {
		if !strings.Contains(v, opt.label) {
			t.Errorf("kind view should contain %q", opt.label)
		}
	}
}

func TestFormModel_ConfirmViewContainsTitle(t *testing.T) {
	v := advanceToConfirm(t).View()
	if !strings.Contains(v, "some title") {
		t.Errorf("confirm view should contain 'some title', got:\n%s", v)
	}
}

func TestFormModel_TitleWarningAppearsInView(t *testing.T) {
	m := advanceToTitle(t)
	m.titleWarning = true
	if v := m.View(); !strings.Contains(v, "必須") {
		t.Errorf("title view with warning should mention 必須, got:\n%s", v)
	}
}

// ─── kindOptions ──────────────────────────────────────────────────────────────

func TestKindOptions_Coverage(t *testing.T) {
	if len(kindOptions) < 2 {
		t.Errorf("kindOptions should have at least 2 entries, got %d", len(kindOptions))
	}
	for i, opt := range kindOptions {
		if opt.icon == "" {
			t.Errorf("kindOptions[%d].icon is empty", i)
		}
		if opt.label == "" {
			t.Errorf("kindOptions[%d].label is empty", i)
		}
	}
}

package ui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// bigEpicIssues is 20 epics of 60 children in every status, so a view
// holds far more cards than fit on screen.
func bigEpicIssues() []model.Issue {
	at := time.Now().Add(-48 * time.Hour)
	var out []model.Issue
	sts := []model.Status{model.StatusOpen, model.StatusInProgress, model.StatusBlocked, model.StatusClosed, model.StatusClosed, model.StatusClosed}
	for e := 0; e < 20; e++ {
		eid := fmt.Sprintf("p-e%d", e)
		out = append(out, model.Issue{ID: eid, Title: "Epic number " + eid + " with a longer title", IssueType: model.TypeEpic, Status: model.StatusOpen, Priority: e % 4, CreatedAt: at, UpdatedAt: at})
		for c := 0; c < 60; c++ {
			id := fmt.Sprintf("%s.%d", eid, c)
			is := model.Issue{ID: id, Title: "Child issue " + id + " does some work here", IssueType: model.TypeTask, Status: sts[c%len(sts)], Priority: c % 4, CreatedAt: at, UpdatedAt: at,
				Dependencies: epicChildDeps(id, eid)}
			if is.Status == model.StatusClosed {
				ct := at
				is.ClosedAt = &ct
			}
			out = append(out, is)
		}
	}
	return out
}

// BenchmarkBoardLaneView measures one keypress on a large board: a move and a redraw.
func BenchmarkBoardLaneView(b *testing.B) {
	for _, v := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		bm := NewBoardModel(bigEpicIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
		bm.SetEpicView(v)
		b.Run(v.String(), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.MoveDown()
				_ = bm.View(200, 50)
			}
		})
	}
}

func BenchmarkBoardModelKeyJ(b *testing.B) {
	m := NewModel(bigEpicIssues(), "")
	u, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	u, _ = u.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = u.(Model)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := "j"
		if i%40 >= 20 {
			k = "k"
		}
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		m = u.(Model)
		_ = m.View()
	}
}

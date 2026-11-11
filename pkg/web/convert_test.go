package web

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestConvertIssueExtraDetailFields(t *testing.T) {
	var src model.Issue
	if err := json.Unmarshal([]byte(`{"id":"bd-39","close_reason":"Finished after review","metadata":{"branch":"feature/planning","nested":{"count":9007199254740993}}}`), &src); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(convertIssue(&src, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Finished after review", "feature/planning", "9007199254740993"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("web detail lost %q: %s", want, raw)
		}
	}
}

// The board's epic cell reads the summary and due date from a snapshot,
// which drops every long text field.
func TestLeanIssuesKeepEpicSummaryAndDue(t *testing.T) {
	due := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := leanIssues([]model.Issue{
		{ID: "x-1", Title: "Checkout", IssueType: model.TypeEpic, Status: model.StatusOpen,
			Description: "# Goal\nShip a faster checkout. Then more.", DueDate: &due,
			CloseReason: "Finished", Metadata: json.RawMessage(`{"branch":"main"}`)},
		{ID: "x-2", Title: "Task", IssueType: model.TypeTask, Status: model.StatusOpen,
			Description: "Not an epic. Stays out."},
	})
	if got[0].Summary != "Ship a faster checkout." {
		t.Errorf("epic summary = %q", got[0].Summary)
	}
	if got[0].Due != "2026-01-02T00:00:00Z" {
		t.Errorf("epic due = %q", got[0].Due)
	}
	if got[0].Description != "" {
		t.Errorf("snapshot kept the description %q", got[0].Description)
	}
	if got[0].CloseReason != "" || got[0].Metadata != "" {
		t.Fatal("snapshot kept extra detail text")
	}
	if got[1].Summary != "" || got[1].Due != "" {
		t.Errorf("task got summary %q due %q", got[1].Summary, got[1].Due)
	}
}

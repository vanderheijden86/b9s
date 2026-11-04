package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildReleases_State(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	past, future, zero := now.Add(-time.Hour), now.Add(time.Hour), time.Time{}
	tests := []struct {
		name      string
		status    Status
		due       *time.Time
		epicState []Status
		want      ReleaseState
	}{
		{"planned without date", StatusOpen, nil, []Status{StatusOpen}, ReleasePlanned},
		{"planned before due date", StatusOpen, &future, []Status{StatusOpen}, ReleasePlanned},
		{"planned at due date", StatusOpen, &now, []Status{StatusOpen}, ReleasePlanned},
		{"planned with zero date", StatusOpen, &zero, []Status{StatusOpen}, ReleasePlanned},
		{"overdue", StatusOpen, &past, []Status{StatusOpen}, ReleaseOverdue},
		{"ready to ship", StatusOpen, &future, []Status{StatusClosed, StatusClosed}, ReleaseReadyToShip},
		{"ready takes precedence over overdue", StatusOpen, &past, []Status{StatusClosed}, ReleaseReadyToShip},
		{"empty scope is ready", StatusOpen, nil, nil, ReleaseReadyToShip},
		{"released with unfinished epic", StatusClosed, &past, []Status{StatusOpen}, ReleaseReleased},
		{"released takes precedence over ready", StatusClosed, &past, []Status{StatusClosed}, ReleaseReleased},
		{"partially closed scope", StatusOpen, nil, []Status{StatusClosed, StatusOpen}, ReleasePlanned},
		{"blocked epic is unfinished", StatusOpen, nil, []Status{StatusBlocked}, ReleasePlanned},
		{"tombstone epic is not closed", StatusOpen, nil, []Status{StatusTombstone}, ReleasePlanned},
		{"in progress milestone is not released", StatusInProgress, &past, []Status{StatusOpen}, ReleaseOverdue},
		{"deferred milestone is not released", StatusDeferred, nil, []Status{StatusOpen}, ReleasePlanned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			milestone := Issue{ID: "release", Title: "v1", IssueType: TypeMilestone, Status: tt.status, DueDate: tt.due}
			issues := []Issue{milestone}
			for i, status := range tt.epicState {
				id := fmt.Sprintf("epic-%d", i)
				issues = append(issues, Issue{ID: id, IssueType: TypeEpic, Status: status})
				issues[0].Dependencies = append(issues[0].Dependencies, &Dependency{
					IssueID: milestone.ID, DependsOnID: id, Type: DepBlocks,
				})
			}
			releases := BuildReleases(issues, now)
			if len(releases) != 1 {
				t.Fatalf("got %d releases, want 1", len(releases))
			}
			if releases[0].State != tt.want {
				t.Errorf("state = %q, want %q", releases[0].State, tt.want)
			}
		})
	}
}

func TestBuildReleases_EpicEdges(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		dep      *Dependency
		target   Issue
		wantEpic bool
		want     ReleaseState
	}{
		{"blocks", &Dependency{IssueID: "release", DependsOnID: "target", Type: DepBlocks}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, true, ReleasePlanned},
		{"legacy blocking edge", &Dependency{IssueID: "release", DependsOnID: "target"}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, true, ReleasePlanned},
		{"related", &Dependency{IssueID: "release", DependsOnID: "target", Type: DepRelated}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"parent child", &Dependency{IssueID: "release", DependsOnID: "target", Type: DepParentChild}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"tracks", &Dependency{IssueID: "release", DependsOnID: "target", Type: "tracks"}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"reverse edge", &Dependency{IssueID: "target", DependsOnID: "release", Type: DepBlocks}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"foreign edge", &Dependency{IssueID: "other", DependsOnID: "target", Type: DepBlocks}, Issue{ID: "target", IssueType: TypeEpic, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"task target", &Dependency{IssueID: "release", DependsOnID: "target", Type: DepBlocks}, Issue{ID: "target", IssueType: TypeTask, Status: StatusOpen}, false, ReleaseReadyToShip},
		{"missing target", &Dependency{IssueID: "release", DependsOnID: "missing", Type: DepBlocks}, Issue{}, false, ReleasePlanned},
		{"nil edge", nil, Issue{}, false, ReleaseReadyToShip},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			milestone := Issue{ID: "release", IssueType: TypeMilestone, Status: StatusOpen, Dependencies: []*Dependency{tt.dep}}
			releases := BuildReleases([]Issue{milestone, tt.target}, now)
			if len(releases) != 1 {
				t.Fatalf("got %d releases, want 1", len(releases))
			}
			release := releases[0]
			if release.State != tt.want {
				t.Errorf("state = %q, want %q", release.State, tt.want)
			}
			if tt.wantEpic {
				if len(release.Epics) != 1 || !reflect.DeepEqual(release.Epics[0], tt.target) {
					t.Errorf("epics = %#v, want [%#v]", release.Epics, tt.target)
				}
			} else if len(release.Epics) != 0 {
				t.Errorf("epics = %#v, want none", release.Epics)
			}
		})
	}
}

func TestBuildReleases_PreservesMilestones(t *testing.T) {
	epic := Issue{ID: "epic", IssueType: TypeEpic, Status: StatusOpen}
	first := Issue{ID: "v2", Title: "Release v2", Description: "notes", IssueType: TypeMilestone, Status: StatusOpen,
		Dependencies: []*Dependency{
			{IssueID: "v2", DependsOnID: epic.ID, Type: DepBlocks},
			{IssueID: "v2", DependsOnID: epic.ID, Type: DepBlocks},
		}}
	second := Issue{ID: "v1", IssueType: TypeMilestone, Status: StatusOpen,
		Dependencies: []*Dependency{{IssueID: "v1", DependsOnID: epic.ID, Type: DepBlocks}}}
	issues := []Issue{first, epic, second, {ID: "task", IssueType: TypeTask}}
	before := make([]Issue, len(issues))
	for i := range issues {
		before[i] = issues[i].Clone()
	}
	releases := BuildReleases(issues, time.Now())
	if len(releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(releases))
	}
	for i, milestone := range []Issue{first, second} {
		if !reflect.DeepEqual(releases[i].Issue, milestone) {
			t.Errorf("milestone %d = %#v, want %#v", i, releases[i].Issue, milestone)
		}
		if len(releases[i].Epics) != 1 || releases[i].Epics[0].ID != epic.ID {
			t.Errorf("epics %d = %#v, want one shared epic", i, releases[i].Epics)
		}
	}
	if !reflect.DeepEqual(issues, before) {
		t.Error("BuildReleases mutated its input")
	}
}

func TestBuildReleases_NoMilestones(t *testing.T) {
	for _, issues := range [][]Issue{nil, {{ID: "epic", IssueType: TypeEpic}}} {
		if releases := BuildReleases(issues, time.Now()); len(releases) != 0 {
			t.Errorf("got %#v, want no releases", releases)
		}
	}
}

func TestReleaseState_IsValid(t *testing.T) {
	for _, state := range []ReleaseState{ReleasePlanned, ReleaseOverdue, ReleaseReadyToShip, ReleaseReleased} {
		if !state.IsValid() {
			t.Errorf("%q should be valid", state)
		}
	}
	for _, state := range []ReleaseState{"", "unknown", "open"} {
		if state.IsValid() {
			t.Errorf("%q should be invalid", state)
		}
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, true},
		{"InProgress", StatusInProgress, true},
		{"Blocked", StatusBlocked, true},
		{"Deferred", StatusDeferred, true},
		{"Pinned", StatusPinned, true},
		{"Hooked", StatusHooked, true},
		{"Review", StatusReview, true},
		{"Closed", StatusClosed, true},
		{"Tombstone", StatusTombstone, true},
		{"Invalid", "unknown", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsClosed(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, false},
		{"InProgress", StatusInProgress, false},
		{"Blocked", StatusBlocked, false},
		{"Closed", StatusClosed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsClosed(); got != tt.want {
				t.Errorf("Status.IsClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsOpen(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, true},
		{"InProgress", StatusInProgress, true},
		{"Blocked", StatusBlocked, false},
		{"Closed", StatusClosed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsOpen(); got != tt.want {
				t.Errorf("Status.IsOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		issueType IssueType
		want      bool
	}{
		{"Bug", TypeBug, true},
		{"Feature", TypeFeature, true},
		{"Task", TypeTask, true},
		{"Epic", TypeEpic, true},
		{"Chore", TypeChore, true},
		// Any non-empty type is valid (extensibility for Beads ecosystem)
		{"CustomType", "custom", true},
		// Gastown orchestration types (steveyegge/beads)
		{"GastownRole", "role", true},
		{"GastownAgent", "agent", true},
		{"GastownMolecule", "molecule", true},
		// Only empty is invalid
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issueType.IsValid(); got != tt.want {
				t.Errorf("IssueType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueType_IsKnownType(t *testing.T) {
	tests := []struct {
		name      string
		issueType IssueType
		want      bool
	}{
		{"Bug", TypeBug, true},
		{"Feature", TypeFeature, true},
		{"Task", TypeTask, true},
		{"Epic", TypeEpic, true},
		{"Chore", TypeChore, true},
		{"Custom", "custom", false},
		{"GastownRole", "role", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issueType.IsKnownType(); got != tt.want {
				t.Errorf("IssueType.IsKnownType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyType_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"Blocks", DepBlocks, true},
		{"Related", DepRelated, true},
		{"ParentChild", DepParentChild, true},
		{"DiscoveredFrom", DepDiscoveredFrom, true},
		{"Invalid", "causes", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.IsValid(); got != tt.want {
				t.Errorf("DependencyType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyType_IsBlocking(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"Blocks", DepBlocks, true},
		{"Related", DepRelated, false},
		{"ParentChild", DepParentChild, false},
		{"Legacy (Empty)", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.IsBlocking(); got != tt.want {
				t.Errorf("DependencyType.IsBlocking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssue_Struct(t *testing.T) {
	// This test verifies that we can construct an Issue with valid data
	now := time.Now()
	issue := &Issue{
		ID:          "TEST-123",
		Title:       "Test Issue",
		Description: "This is a test issue",
		Status:      StatusOpen,
		Priority:    1, // lower is higher priority
		IssueType:   TypeTask,
		CreatedAt:   now,
		UpdatedAt:   now,
		Labels:      []string{"test", "unit"},
	}

	if issue.ID != "TEST-123" {
		t.Errorf("Issue ID mismatch: got %s, want TEST-123", issue.ID)
	}
	if !issue.Status.IsValid() {
		t.Errorf("Issue Status should be valid")
	}
	if !issue.IssueType.IsValid() {
		t.Errorf("Issue Type should be valid")
	}

	// UpdatedAt should never be before CreatedAt in valid data
	if issue.UpdatedAt.Before(issue.CreatedAt) {
		t.Errorf("UpdatedAt should be >= CreatedAt")
	}
}

func TestDependency_Struct(t *testing.T) {
	now := time.Now()
	dep := &Dependency{
		IssueID:     "A",
		DependsOnID: "B",
		Type:        DepBlocks,
		CreatedAt:   now,
		CreatedBy:   "user",
	}

	if dep.IssueID != "A" {
		t.Errorf("IssueID mismatch")
	}
	if !dep.Type.IsValid() {
		t.Errorf("Dependency type should be valid")
	}
	if !dep.Type.IsBlocking() {
		t.Errorf("DepBlocks should be blocking")
	}
}

func TestComment_Struct(t *testing.T) {
	now := time.Now()
	comment := &Comment{
		ID:        "1",
		IssueID:   "A",
		Author:    "user",
		Text:      "hello",
		CreatedAt: now,
	}

	if comment.IssueID != "A" {
		t.Errorf("IssueID mismatch")
	}
	if comment.Text != "hello" {
		t.Errorf("Text mismatch")
	}
}

func TestIssue_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		issue   Issue
		wantErr bool
	}{
		{
			name: "Valid",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "Valid Issue",
				Status:    StatusOpen,
				IssueType: TypeBug,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: false,
		},
		{
			name: "Empty ID",
			issue: Issue{
				ID:        "",
				Title:     "Valid Issue",
				Status:    StatusOpen,
				IssueType: TypeBug,
			},
			wantErr: true,
		},
		{
			name: "Empty Title",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "",
				Status:    StatusOpen,
				IssueType: TypeBug,
			},
			wantErr: true,
		},
		{
			name: "Invalid Status",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "Valid Issue",
				Status:    "invalid",
				IssueType: TypeBug,
			},
			wantErr: true,
		},
		{
			name: "Empty Type",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "Valid Issue",
				Status:    StatusOpen,
				IssueType: "", // Only empty type is invalid
			},
			wantErr: true,
		},
		{
			name: "Custom Type Allowed",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "Valid Issue",
				Status:    StatusOpen,
				IssueType: "gastown-role", // Non-standard types are now valid
			},
			wantErr: false,
		},
		{
			name: "UpdatedAt Before CreatedAt",
			issue: Issue{
				ID:        "TEST-1",
				Title:     "Valid Issue",
				Status:    StatusOpen,
				IssueType: TypeBug,
				CreatedAt: now,
				UpdatedAt: now.Add(-1 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Issue.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForecast_Validate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		forecast Forecast
		wantErr  bool
	}{
		{
			name: "Valid",
			forecast: Forecast{
				BeadID:     "bv-123",
				ETADate:    now.Add(24 * time.Hour),
				Confidence: 0.7,
			},
			wantErr: false,
		},
		{
			name: "Empty BeadID",
			forecast: Forecast{
				BeadID:     "",
				ETADate:    now.Add(24 * time.Hour),
				Confidence: 0.7,
			},
			wantErr: true,
		},
		{
			name: "Zero ETADate",
			forecast: Forecast{
				BeadID:     "bv-123",
				ETADate:    time.Time{},
				Confidence: 0.7,
			},
			wantErr: true,
		},
		{
			name: "Confidence Out Of Range",
			forecast: Forecast{
				BeadID:     "bv-123",
				ETADate:    now.Add(24 * time.Hour),
				Confidence: 1.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.forecast.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Forecast.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForecast_JSON(t *testing.T) {
	eta := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	created := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	f := Forecast{
		BeadID:     "bv-123",
		ETADate:    eta,
		Confidence: 0.42,
		Factors:    []string{"label=backend"},
		CreatedAt:  created,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTrip Forecast
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if roundTrip.BeadID != f.BeadID {
		t.Errorf("BeadID mismatch: got %q, want %q", roundTrip.BeadID, f.BeadID)
	}
	if !roundTrip.ETADate.Equal(f.ETADate) {
		t.Errorf("ETADate mismatch: got %v, want %v", roundTrip.ETADate, f.ETADate)
	}
	if roundTrip.Confidence != f.Confidence {
		t.Errorf("Confidence mismatch: got %v, want %v", roundTrip.Confidence, f.Confidence)
	}
	if len(roundTrip.Factors) != 1 || roundTrip.Factors[0] != "label=backend" {
		t.Errorf("Factors mismatch: got %#v", roundTrip.Factors)
	}
	if !roundTrip.CreatedAt.Equal(f.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", roundTrip.CreatedAt, f.CreatedAt)
	}

	empty := Forecast{
		BeadID:     "bv-123",
		ETADate:    eta,
		Confidence: 0.42,
	}
	emptyJSON, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(emptyJSON), "factors") {
		t.Errorf("Expected factors to be omitted when empty: %s", emptyJSON)
	}
	// Note: time.Time with omitempty doesn't omit zero values in Go's JSON encoder.
	// This is a Go limitation - struct types are never considered "empty" for omitempty.
}

func TestBurndownPoint_Validate(t *testing.T) {
	d := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		point   BurndownPoint
		wantErr bool
	}{
		{
			name: "Valid",
			point: BurndownPoint{
				Date:      d,
				Remaining: 10,
				Completed: 5,
			},
			wantErr: false,
		},
		{
			name: "Zero Date",
			point: BurndownPoint{
				Date:      time.Time{},
				Remaining: 10,
				Completed: 5,
			},
			wantErr: true,
		},
		{
			name: "Negative Remaining",
			point: BurndownPoint{
				Date:      d,
				Remaining: -1,
				Completed: 0,
			},
			wantErr: true,
		},
		{
			name: "Negative Completed",
			point: BurndownPoint{
				Date:      d,
				Remaining: 0,
				Completed: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.point.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BurndownPoint.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBurndownPoint_JSON(t *testing.T) {
	p := BurndownPoint{
		Date:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Remaining: 10,
		Completed: 5,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTrip BurndownPoint
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !roundTrip.Date.Equal(p.Date) || roundTrip.Remaining != p.Remaining || roundTrip.Completed != p.Completed {
		t.Errorf("Round-trip mismatch: got %#v, want %#v", roundTrip, p)
	}
}

func TestForecast_Struct(t *testing.T) {
	now := time.Now()
	eta := now.AddDate(0, 0, 5)

	forecast := Forecast{
		BeadID:     "bv-123",
		ETADate:    eta,
		Confidence: 0.75,
		Factors:    []string{"estimate: explicit (120m)", "type: task×1.0"},
		CreatedAt:  now,
	}

	if forecast.BeadID != "bv-123" {
		t.Errorf("BeadID mismatch: got %s", forecast.BeadID)
	}
	if forecast.Confidence != 0.75 {
		t.Errorf("Confidence mismatch: got %f", forecast.Confidence)
	}
	if len(forecast.Factors) != 2 {
		t.Errorf("Factors length mismatch: got %d, want 2", len(forecast.Factors))
	}
	if !forecast.ETADate.Equal(eta) {
		t.Errorf("ETADate mismatch: got %v, want %v", forecast.ETADate, eta)
	}
}

func TestBurndownPoint_Struct(t *testing.T) {
	now := time.Now()

	point := BurndownPoint{
		Date:      now,
		Remaining: 15,
		Completed: 10,
	}

	if point.Remaining != 15 {
		t.Errorf("Remaining mismatch: got %d", point.Remaining)
	}
	if point.Completed != 10 {
		t.Errorf("Completed mismatch: got %d", point.Completed)
	}
	if !point.Date.Equal(now) {
		t.Errorf("Date mismatch")
	}
}

func TestIssue_Clone(t *testing.T) {
	now := time.Now()
	closedAt := now.Add(-1 * time.Hour)
	estimatedMinutes := 60
	externalRef := "JIRA-123"
	compactedAt := now.Add(-2 * time.Hour)
	compactedAtCommit := "abc123"

	original := Issue{
		ID:                "TEST-1",
		Title:             "Test Issue",
		Description:       "Description",
		Status:            StatusOpen,
		Priority:          1,
		IssueType:         TypeBug,
		Assignee:          "user",
		EstimatedMinutes:  &estimatedMinutes,
		CreatedAt:         now,
		UpdatedAt:         now,
		ClosedAt:          &closedAt,
		ExternalRef:       &externalRef,
		CompactedAt:       &compactedAt,
		CompactedAtCommit: &compactedAtCommit,
		Labels:            []string{"bug", "critical"},
		Dependencies: []*Dependency{
			{IssueID: "TEST-1", DependsOnID: "TEST-2", Type: DepBlocks},
		},
		Comments: []*Comment{
			{ID: "1", IssueID: "TEST-1", Author: "user", Text: "comment"},
		},
	}

	clone := original.Clone()

	// Verify basic field equality
	if clone.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if clone.Title != original.Title {
		t.Errorf("Title mismatch")
	}

	// Verify pointer fields are deep copied
	if clone.EstimatedMinutes == original.EstimatedMinutes {
		t.Errorf("EstimatedMinutes should be a new pointer")
	}
	if *clone.EstimatedMinutes != *original.EstimatedMinutes {
		t.Errorf("EstimatedMinutes value mismatch")
	}

	if clone.ClosedAt == original.ClosedAt {
		t.Errorf("ClosedAt should be a new pointer")
	}
	if !clone.ClosedAt.Equal(*original.ClosedAt) {
		t.Errorf("ClosedAt value mismatch")
	}

	// Verify slice fields are deep copied
	if &clone.Labels == &original.Labels {
		t.Errorf("Labels should be a new slice")
	}
	if len(clone.Labels) != len(original.Labels) {
		t.Errorf("Labels length mismatch")
	}

	// Verify modifying clone doesn't affect original
	*clone.EstimatedMinutes = 120
	if *original.EstimatedMinutes != 60 {
		t.Errorf("Modifying clone affected original EstimatedMinutes")
	}

	clone.Labels[0] = "modified"
	if original.Labels[0] != "bug" {
		t.Errorf("Modifying clone affected original Labels")
	}

	// Verify Dependencies are deep copied
	if len(clone.Dependencies) != 1 {
		t.Errorf("Dependencies length mismatch")
	}
	if clone.Dependencies[0] == original.Dependencies[0] {
		t.Errorf("Dependencies[0] should be a new pointer")
	}

	// Verify Comments are deep copied
	if len(clone.Comments) != 1 {
		t.Errorf("Comments length mismatch")
	}
	if clone.Comments[0] == original.Comments[0] {
		t.Errorf("Comments[0] should be a new pointer")
	}
}

func TestIssue_Clone_NilFields(t *testing.T) {
	original := Issue{
		ID:        "TEST-1",
		Title:     "Test",
		Status:    StatusOpen,
		IssueType: TypeTask,
	}

	clone := original.Clone()

	if clone.EstimatedMinutes != nil {
		t.Errorf("EstimatedMinutes should be nil")
	}
	if clone.ClosedAt != nil {
		t.Errorf("ClosedAt should be nil")
	}
	if clone.Labels != nil {
		t.Errorf("Labels should be nil")
	}
	if clone.Dependencies != nil {
		t.Errorf("Dependencies should be nil")
	}
	if clone.Comments != nil {
		t.Errorf("Comments should be nil")
	}
}

func TestIssueUnmarshalReadsDeferUntil(t *testing.T) {
	var issue Issue
	data := []byte(`{"id":"x-1","title":"t","status":"deferred","defer_until":"2026-10-01T00:00:00Z"}`)
	if err := json.Unmarshal(data, &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.DeferUntil == nil {
		t.Fatal("DeferUntil is nil, want 2026-10-01")
	}
	if got := issue.DeferUntil.Format("2006-01-02"); got != "2026-10-01" {
		t.Errorf("DeferUntil = %s, want 2026-10-01", got)
	}
}

func TestIssueUnmarshal_ReadsCreatorAndOwner(t *testing.T) {
	var issue Issue
	line := `{"id":"bd-1","title":"t","created_by":"alice","owner":"v@example.com","assignee":"BrownBear"}`
	if err := json.Unmarshal([]byte(line), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.CreatedBy != "alice" {
		t.Errorf("CreatedBy = %q, want alice", issue.CreatedBy)
	}
	if issue.Owner != "v@example.com" {
		t.Errorf("Owner = %q, want v@example.com", issue.Owner)
	}
	if issue.Assignee != "BrownBear" {
		t.Errorf("Assignee = %q, want BrownBear (creator and assignee stay separate)", issue.Assignee)
	}
}

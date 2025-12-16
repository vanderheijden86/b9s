package analysis

import (
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestDefaultAdvancedInsightsConfig(t *testing.T) {
	cfg := DefaultAdvancedInsightsConfig()

	if cfg.TopKSetLimit != 5 {
		t.Errorf("TopKSetLimit: expected 5, got %d", cfg.TopKSetLimit)
	}
	if cfg.CoverageSetLimit != 5 {
		t.Errorf("CoverageSetLimit: expected 5, got %d", cfg.CoverageSetLimit)
	}
	if cfg.KPathsLimit != 5 {
		t.Errorf("KPathsLimit: expected 5, got %d", cfg.KPathsLimit)
	}
	if cfg.PathLengthCap != 50 {
		t.Errorf("PathLengthCap: expected 50, got %d", cfg.PathLengthCap)
	}
	if cfg.CycleBreakLimit != 5 {
		t.Errorf("CycleBreakLimit: expected 5, got %d", cfg.CycleBreakLimit)
	}
	if cfg.ParallelCutLimit != 5 {
		t.Errorf("ParallelCutLimit: expected 5, got %d", cfg.ParallelCutLimit)
	}
}

func TestDefaultUsageHints(t *testing.T) {
	hints := DefaultUsageHints()

	expected := []string{"topk_set", "coverage_set", "k_paths", "parallel_cut", "parallel_gain", "cycle_break"}
	for _, key := range expected {
		if hints[key] == "" {
			t.Errorf("Missing usage hint for %s", key)
		}
	}
}

func TestGenerateAdvancedInsightsEmpty(t *testing.T) {
	an := NewAnalyzer([]model.Issue{})
	cfg := DefaultAdvancedInsightsConfig()
	insights := an.GenerateAdvancedInsights(cfg)

	if insights == nil {
		t.Fatal("expected non-nil insights")
	}
	if insights.Config.TopKSetLimit != 5 {
		t.Error("config not preserved")
	}
	if len(insights.UsageHints) == 0 {
		t.Error("expected usage hints")
	}

	// All features should have status
	if insights.TopKSet == nil || insights.TopKSet.Status.State == "" {
		t.Error("TopKSet missing or no status")
	}
	if insights.CoverageSet == nil || insights.CoverageSet.Status.State == "" {
		t.Error("CoverageSet missing or no status")
	}
	if insights.KPaths == nil || insights.KPaths.Status.State == "" {
		t.Error("KPaths missing or no status")
	}
	if insights.ParallelCut == nil || insights.ParallelCut.Status.State == "" {
		t.Error("ParallelCut missing or no status")
	}
	if insights.ParallelGain == nil || insights.ParallelGain.Status.State == "" {
		t.Error("ParallelGain missing or no status")
	}
	if insights.CycleBreak == nil || insights.CycleBreak.Status.State == "" {
		t.Error("CycleBreak missing or no status")
	}
}

func TestGenerateAdvancedInsightsNoCycles(t *testing.T) {
	// Linear chain with no cycles
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	insights := an.GenerateAdvancedInsights(cfg)

	// Cycle break should report no cycles
	if insights.CycleBreak == nil {
		t.Fatal("expected CycleBreak result")
	}
	if insights.CycleBreak.Status.State != "available" {
		t.Errorf("expected available state, got %s", insights.CycleBreak.Status.State)
	}
	if insights.CycleBreak.CycleCount != 0 {
		t.Errorf("expected 0 cycles, got %d", insights.CycleBreak.CycleCount)
	}
	if len(insights.CycleBreak.Suggestions) != 0 {
		t.Error("expected no suggestions for acyclic graph")
	}
}

func TestGenerateAdvancedInsightsWithCycles(t *testing.T) {
	// Create a cycle: A -> B -> C -> A
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	insights := an.GenerateAdvancedInsights(cfg)

	// Cycle break should detect the cycle
	if insights.CycleBreak == nil {
		t.Fatal("expected CycleBreak result")
	}
	if insights.CycleBreak.Status.State != "available" {
		t.Errorf("expected available state, got %s", insights.CycleBreak.Status.State)
	}
	if insights.CycleBreak.CycleCount == 0 {
		t.Error("expected cycles to be detected")
	}
	if len(insights.CycleBreak.Suggestions) == 0 {
		t.Error("expected cycle break suggestions")
	}
	if insights.CycleBreak.Advisory == "" {
		t.Error("expected advisory text")
	}
}

func TestCycleBreakSuggestionsCapping(t *testing.T) {
	// Create multiple cycles by having a hub with many back-edges
	issues := []model.Issue{
		{ID: "Hub", Status: model.StatusOpen},
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		// Create back-edges to form cycles
		{ID: "Hub2", Status: model.StatusOpen, Dependencies: []*model.Dependency{
			{DependsOnID: "A", Type: model.DepBlocks},
			{DependsOnID: "B", Type: model.DepBlocks},
			{DependsOnID: "C", Type: model.DepBlocks},
		}},
	}
	// Add edge from Hub to Hub2 to complete cycles
	issues[0].Dependencies = []*model.Dependency{{DependsOnID: "Hub2", Type: model.DepBlocks}}

	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	cfg.CycleBreakLimit = 2 // Low cap for testing
	insights := an.GenerateAdvancedInsights(cfg)

	if insights.CycleBreak == nil {
		t.Fatal("expected CycleBreak result")
	}
	if len(insights.CycleBreak.Suggestions) > 2 {
		t.Errorf("expected at most 2 suggestions (capped), got %d", len(insights.CycleBreak.Suggestions))
	}
}

func TestCycleBreakDeterministic(t *testing.T) {
	// Run multiple times and verify deterministic output
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}

	cfg := DefaultAdvancedInsightsConfig()
	var firstResult *CycleBreakResult

	for i := 0; i < 5; i++ {
		an := NewAnalyzer(issues)
		insights := an.GenerateAdvancedInsights(cfg)

		if firstResult == nil {
			firstResult = insights.CycleBreak
			continue
		}

		// Compare with first result
		if len(insights.CycleBreak.Suggestions) != len(firstResult.Suggestions) {
			t.Fatalf("iteration %d: suggestion count changed", i)
		}
		for j, s := range insights.CycleBreak.Suggestions {
			if s.EdgeFrom != firstResult.Suggestions[j].EdgeFrom || s.EdgeTo != firstResult.Suggestions[j].EdgeTo {
				t.Errorf("iteration %d: suggestion %d order changed", i, j)
			}
		}
	}
}

func TestPendingFeatureStatus(t *testing.T) {
	issues := []model.Issue{{ID: "A", Status: model.StatusOpen}}
	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	insights := an.GenerateAdvancedInsights(cfg)

	// Features that are still pending (awaiting implementation)
	pendingFeatures := []struct {
		name   string
		status FeatureStatus
	}{
		{"KPaths", insights.KPaths.Status},
		{"ParallelCut", insights.ParallelCut.Status},
		{"ParallelGain", insights.ParallelGain.Status},
	}

	for _, f := range pendingFeatures {
		if f.status.State != "pending" {
			t.Errorf("%s: expected pending state, got %s", f.name, f.status.State)
		}
		if f.status.Reason == "" {
			t.Errorf("%s: expected reason for pending state", f.name)
		}
	}

	// CycleBreak should be available
	if insights.CycleBreak.Status.State != "available" {
		t.Errorf("CycleBreak: expected available state, got %s", insights.CycleBreak.Status.State)
	}

	// TopKSet should be available (bv-145)
	if insights.TopKSet.Status.State != "available" {
		t.Errorf("TopKSet: expected available state, got %s", insights.TopKSet.Status.State)
	}

	// CoverageSet should be available (bv-152)
	if insights.CoverageSet.Status.State != "available" {
		t.Errorf("CoverageSet: expected available state, got %s", insights.CoverageSet.Status.State)
	}
}

func TestTopKSetEmpty(t *testing.T) {
	an := NewAnalyzer([]model.Issue{})
	cfg := DefaultAdvancedInsightsConfig()
	insights := an.GenerateAdvancedInsights(cfg)

	if insights.TopKSet == nil {
		t.Fatal("expected TopKSet result")
	}
	if insights.TopKSet.Status.State != "available" {
		t.Errorf("expected available state, got %s", insights.TopKSet.Status.State)
	}
	if len(insights.TopKSet.Items) != 0 {
		t.Error("expected no items for empty graph")
	}
	if insights.TopKSet.TotalGain != 0 {
		t.Errorf("expected 0 total gain, got %d", insights.TopKSet.TotalGain)
	}
}

func TestTopKSetLinearChain(t *testing.T) {
	// A -> B -> C -> D: completing A unblocks B, completing B unblocks C, etc.
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	cfg.TopKSetLimit = 2
	insights := an.GenerateAdvancedInsights(cfg)

	if insights.TopKSet == nil {
		t.Fatal("expected TopKSet result")
	}
	if insights.TopKSet.Status.State != "available" {
		t.Errorf("expected available state, got %s", insights.TopKSet.Status.State)
	}
	// First pick should be A (unblocks B)
	if len(insights.TopKSet.Items) < 1 {
		t.Fatal("expected at least 1 item")
	}
	if insights.TopKSet.Items[0].ID != "A" {
		t.Errorf("first pick should be A, got %s", insights.TopKSet.Items[0].ID)
	}
	if insights.TopKSet.Items[0].MarginalGain != 1 {
		t.Errorf("A should unblock 1 (B), got %d", insights.TopKSet.Items[0].MarginalGain)
	}
	// Second pick should be B (unblocks C)
	if len(insights.TopKSet.Items) < 2 {
		t.Fatal("expected 2 items")
	}
	if insights.TopKSet.Items[1].ID != "B" {
		t.Errorf("second pick should be B, got %s", insights.TopKSet.Items[1].ID)
	}
}

func TestTopKSetDeterministic(t *testing.T) {
	issues := []model.Issue{
		{ID: "Hub", Status: model.StatusOpen},
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
	}

	cfg := DefaultAdvancedInsightsConfig()
	var firstResult *TopKSetResult

	for i := 0; i < 5; i++ {
		an := NewAnalyzer(issues)
		insights := an.GenerateAdvancedInsights(cfg)

		if firstResult == nil {
			firstResult = insights.TopKSet
			continue
		}

		// Compare with first result
		if len(insights.TopKSet.Items) != len(firstResult.Items) {
			t.Fatalf("iteration %d: item count changed", i)
		}
		for j, item := range insights.TopKSet.Items {
			if item.ID != firstResult.Items[j].ID {
				t.Errorf("iteration %d: item %d ID changed from %s to %s", i, j, firstResult.Items[j].ID, item.ID)
			}
		}
	}
}

func TestTopKSetCapping(t *testing.T) {
	// Create more items than the cap
	issues := []model.Issue{
		{ID: "Hub", Status: model.StatusOpen},
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "E", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
		{ID: "F", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "Hub", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	cfg := DefaultAdvancedInsightsConfig()
	cfg.TopKSetLimit = 3
	insights := an.GenerateAdvancedInsights(cfg)

	if len(insights.TopKSet.Items) > 3 {
		t.Errorf("expected at most 3 items (capped), got %d", len(insights.TopKSet.Items))
	}
	if !insights.TopKSet.Status.Capped {
		t.Error("expected Capped=true when results exceed limit")
	}
}

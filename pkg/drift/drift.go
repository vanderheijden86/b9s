// Package drift provides drift detection by comparing current metrics to a baseline.
// It identifies changes in graph structure, cycles, and key metrics.
package drift

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/baseline"
)

// Severity represents the severity level of a drift alert
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// AlertType categorizes different kinds of drift alerts
type AlertType string

const (
	AlertNewCycle           AlertType = "new_cycle"
	AlertPageRankChange     AlertType = "pagerank_change"
	AlertDensityGrowth      AlertType = "density_growth"
	AlertNodeCountChange    AlertType = "node_count_change"
	AlertEdgeCountChange    AlertType = "edge_count_change"
	AlertBlockedIncrease    AlertType = "blocked_increase"
	AlertActionableChange   AlertType = "actionable_change"
	AlertStaleIssue         AlertType = "stale_issue"
	AlertVelocityDrop       AlertType = "velocity_drop"
	AlertBlockingCascade    AlertType = "blocking_cascade"
	AlertHighImpactUnblock  AlertType = "high_impact_unblock"
	AlertAbandonedClaim     AlertType = "abandoned_claim"
	AlertPotentialDuplicate AlertType = "potential_duplicate"
)

// Alert represents a single drift detection alert
type Alert struct {
	Type        AlertType `json:"type"`
	Severity    Severity  `json:"severity"`
	Message     string    `json:"message"`
	BaselineVal float64   `json:"baseline_value,omitempty"`
	CurrentVal  float64   `json:"current_value,omitempty"`
	Delta       float64   `json:"delta,omitempty"`
	Details     []string  `json:"details,omitempty"`
	IssueID     string    `json:"issue_id,omitempty"`
	Label       string    `json:"label,omitempty"`
	DetectedAt  time.Time `json:"detected_at,omitempty"`
}

// Result contains the complete drift analysis
type Result struct {
	// HasDrift is true if any alerts were generated
	HasDrift bool `json:"has_drift"`

	// Alerts lists all detected drift issues
	Alerts []Alert `json:"alerts"`

	// Summary statistics
	CriticalCount int `json:"critical_count"`
	WarningCount  int `json:"warning_count"`
	InfoCount     int `json:"info_count"`
}

// Calculator performs drift detection
type Calculator struct {
	config   *Config
	baseline *baseline.Baseline
	current  *baseline.Baseline
}

// NewCalculator creates a drift calculator with the given baseline and current snapshot
func NewCalculator(bl *baseline.Baseline, current *baseline.Baseline, cfg *Config) *Calculator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Calculator{
		config:   cfg,
		baseline: bl,
		current:  current,
	}
}

// Calculate performs drift detection and returns results
func (c *Calculator) Calculate() *Result {
	result := &Result{
		Alerts: make([]Alert, 0),
	}

	// Check for new cycles (critical)
	c.checkCycles(result)

	// Check density growth (info/warning)
	c.checkDensity(result)

	// Check node/edge count changes (info)
	c.checkGraphSize(result)

	// Check blocked issues increase (warning)
	c.checkBlocked(result)

	// Check actionable count changes (info)
	c.checkActionable(result)

	// Check PageRank changes (warning)
	c.checkPageRankChanges(result)

	// Compute summary
	for _, alert := range result.Alerts {
		switch alert.Severity {
		case SeverityCritical:
			result.CriticalCount++
		case SeverityWarning:
			result.WarningCount++
		case SeverityInfo:
			result.InfoCount++
		}
	}
	result.HasDrift = len(result.Alerts) > 0

	return result
}

// checkCycles detects new cycles that weren't in the baseline
func (c *Calculator) checkCycles(result *Result) {
	baselineCycles := make(map[string]bool)
	for _, cycle := range c.baseline.Cycles {
		key := cycleKey(cycle)
		baselineCycles[key] = true
	}

	var newCycles [][]string
	for _, cycle := range c.current.Cycles {
		key := cycleKey(cycle)
		if !baselineCycles[key] {
			newCycles = append(newCycles, cycle)
		}
	}

	if len(newCycles) > 0 {
		details := make([]string, 0, len(newCycles))
		for _, cycle := range newCycles {
			details = append(details, strings.Join(cycle, " → "))
		}

		result.Alerts = append(result.Alerts, Alert{
			Type:        AlertNewCycle,
			Severity:    SeverityCritical,
			Message:     fmt.Sprintf("%d new cycle(s) detected", len(newCycles)),
			BaselineVal: float64(len(c.baseline.Cycles)),
			CurrentVal:  float64(len(c.current.Cycles)),
			Delta:       float64(len(newCycles)),
			Details:     details,
			DetectedAt:  time.Now().UTC(),
		})
	}
}

// checkDensity checks for significant density changes
func (c *Calculator) checkDensity(result *Result) {
	blDensity := c.baseline.Stats.Density
	curDensity := c.current.Stats.Density

	if blDensity == 0 {
		return // No baseline to compare
	}

	delta := curDensity - blDensity
	pctChange := (delta / blDensity) * 100

	if pctChange >= c.config.DensityWarningPct {
		result.Alerts = append(result.Alerts, Alert{
			Type:        AlertDensityGrowth,
			Severity:    SeverityWarning,
			Message:     fmt.Sprintf("Graph density increased by %.1f%%", pctChange),
			BaselineVal: blDensity,
			CurrentVal:  curDensity,
			Delta:       delta,
			DetectedAt:  time.Now().UTC(),
		})
	} else if pctChange >= c.config.DensityInfoPct {
		result.Alerts = append(result.Alerts, Alert{
			Type:        AlertDensityGrowth,
			Severity:    SeverityInfo,
			Message:     fmt.Sprintf("Graph density increased by %.1f%%", pctChange),
			BaselineVal: blDensity,
			CurrentVal:  curDensity,
			Delta:       delta,
			DetectedAt:  time.Now().UTC(),
		})
	}
}

// checkGraphSize checks for significant node/edge count changes
func (c *Calculator) checkGraphSize(result *Result) {
	blNodes := c.baseline.Stats.NodeCount
	curNodes := c.current.Stats.NodeCount
	nodeDelta := curNodes - blNodes

	if blNodes > 0 {
		nodePct := float64(nodeDelta) / float64(blNodes) * 100
		if nodePct >= c.config.NodeGrowthInfoPct || nodePct <= -c.config.NodeGrowthInfoPct {
			result.Alerts = append(result.Alerts, Alert{
				Type:        AlertNodeCountChange,
				Severity:    SeverityInfo,
				Message:     fmt.Sprintf("Node count changed by %+d (%.1f%%)", nodeDelta, nodePct),
				BaselineVal: float64(blNodes),
				CurrentVal:  float64(curNodes),
				Delta:       float64(nodeDelta),
				DetectedAt:  time.Now().UTC(),
			})
		}
	}

	blEdges := c.baseline.Stats.EdgeCount
	curEdges := c.current.Stats.EdgeCount
	edgeDelta := curEdges - blEdges

	if blEdges > 0 {
		edgePct := float64(edgeDelta) / float64(blEdges) * 100
		if edgePct >= c.config.EdgeGrowthInfoPct || edgePct <= -c.config.EdgeGrowthInfoPct {
			result.Alerts = append(result.Alerts, Alert{
				Type:        AlertEdgeCountChange,
				Severity:    SeverityInfo,
				Message:     fmt.Sprintf("Edge count changed by %+d (%.1f%%)", edgeDelta, edgePct),
				BaselineVal: float64(blEdges),
				CurrentVal:  float64(curEdges),
				Delta:       float64(edgeDelta),
				DetectedAt:  time.Now().UTC(),
			})
		}
	}
}

// checkBlocked checks for increases in blocked issues
func (c *Calculator) checkBlocked(result *Result) {
	blBlocked := c.baseline.Stats.BlockedCount
	curBlocked := c.current.Stats.BlockedCount
	delta := curBlocked - blBlocked

	if delta >= c.config.BlockedIncreaseThreshold {
		result.Alerts = append(result.Alerts, Alert{
			Type:        AlertBlockedIncrease,
			Severity:    SeverityWarning,
			Message:     fmt.Sprintf("Blocked issues increased by %d", delta),
			BaselineVal: float64(blBlocked),
			CurrentVal:  float64(curBlocked),
			Delta:       float64(delta),
			DetectedAt:  time.Now().UTC(),
		})
	}
}

// checkActionable checks for significant changes in actionable issues
func (c *Calculator) checkActionable(result *Result) {
	blAction := c.baseline.Stats.ActionableCount
	curAction := c.current.Stats.ActionableCount
	delta := curAction - blAction

	if blAction > 0 {
		pct := float64(delta) / float64(blAction) * 100
		if pct <= -c.config.ActionableDecreaseWarningPct {
			result.Alerts = append(result.Alerts, Alert{
				Type:        AlertActionableChange,
				Severity:    SeverityWarning,
				Message:     fmt.Sprintf("Actionable issues decreased by %d (%.1f%%)", -delta, -pct),
				BaselineVal: float64(blAction),
				CurrentVal:  float64(curAction),
				Delta:       float64(delta),
				DetectedAt:  time.Now().UTC(),
			})
		} else if pct >= c.config.ActionableIncreaseInfoPct || pct <= -c.config.ActionableIncreaseInfoPct {
			result.Alerts = append(result.Alerts, Alert{
				Type:        AlertActionableChange,
				Severity:    SeverityInfo,
				Message:     fmt.Sprintf("Actionable issues changed by %+d (%.1f%%)", delta, pct),
				BaselineVal: float64(blAction),
				CurrentVal:  float64(curAction),
				Delta:       float64(delta),
				DetectedAt:  time.Now().UTC(),
			})
		}
	}
}

// checkPageRankChanges detects significant changes in top PageRank items
func (c *Calculator) checkPageRankChanges(result *Result) {
	blPR := make(map[string]float64)
	for _, item := range c.baseline.TopMetrics.PageRank {
		blPR[item.ID] = item.Value
	}

	curPR := make(map[string]float64)
	for _, item := range c.current.TopMetrics.PageRank {
		curPR[item.ID] = item.Value
	}

	var changes []string

	// Check for significant changes in existing items
	for id, blVal := range blPR {
		curVal, exists := curPR[id]
		if !exists {
			changes = append(changes, fmt.Sprintf("%s dropped from top", id))
			continue
		}
		if blVal > 0 {
			pctChange := ((curVal - blVal) / blVal) * 100
			if pctChange >= c.config.PageRankChangeWarningPct || pctChange <= -c.config.PageRankChangeWarningPct {
				changes = append(changes, fmt.Sprintf("%s: %.1f%% change", id, pctChange))
			}
		}
	}

	// Check for new entries in top
	for id := range curPR {
		if _, exists := blPR[id]; !exists {
			changes = append(changes, fmt.Sprintf("%s entered top", id))
		}
	}

	if len(changes) > 0 {
		result.Alerts = append(result.Alerts, Alert{
			Type:       AlertPageRankChange,
			Severity:   SeverityWarning,
			Message:    fmt.Sprintf("%d PageRank changes detected", len(changes)),
			Details:    changes,
			DetectedAt: time.Now().UTC(),
		})
	}
}

// cycleKey creates a normalized key for a cycle for comparison.
// Note: This uses sorted elements which works when cycle detection algorithms
// produce consistent starting points. If cycles could start at different nodes
// (e.g., [A,B,C,A] vs [B,C,A,B]), a more sophisticated normalization would be
// needed (e.g., canonical rotation to smallest element first).
func cycleKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	// Sort to normalize - assumes cycle detection produces consistent representations
	sorted := make([]string, len(cycle))
	copy(sorted, cycle)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}

// Summary returns a human-readable summary of drift results
func (r *Result) Summary() string {
	if !r.HasDrift {
		return "No drift detected. Project metrics are within baseline thresholds.\n"
	}

	var sb strings.Builder
	sb.WriteString("Drift Analysis Summary\n")
	sb.WriteString("======================\n\n")

	if r.CriticalCount > 0 {
		sb.WriteString(fmt.Sprintf("🔴 CRITICAL: %d issue(s)\n", r.CriticalCount))
	}
	if r.WarningCount > 0 {
		sb.WriteString(fmt.Sprintf("🟡 WARNING: %d issue(s)\n", r.WarningCount))
	}
	if r.InfoCount > 0 {
		sb.WriteString(fmt.Sprintf("🔵 INFO: %d issue(s)\n", r.InfoCount))
	}

	sb.WriteString("\nDetails:\n")
	for _, alert := range r.Alerts {
		icon := "ℹ️"
		switch alert.Severity {
		case SeverityCritical:
			icon = "🔴"
		case SeverityWarning:
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", icon, alert.Type, alert.Message))
		for _, detail := range alert.Details {
			sb.WriteString(fmt.Sprintf("      - %s\n", detail))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// HasCritical returns true if there are any critical alerts
func (r *Result) HasCritical() bool {
	return r.CriticalCount > 0
}

// HasWarnings returns true if there are any warning or critical alerts
func (r *Result) HasWarnings() bool {
	return r.CriticalCount > 0 || r.WarningCount > 0
}

// ExitCode returns suggested exit code for CI use
// 0 = no drift, 1 = critical, 2 = warning, 0 = info only
func (r *Result) ExitCode() int {
	if r.CriticalCount > 0 {
		return 1
	}
	if r.WarningCount > 0 {
		return 2
	}
	return 0
}

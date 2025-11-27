package recipe

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Recipe defines a reusable view configuration for beads
type Recipe struct {
	Name        string       `yaml:"name" json:"name"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Filters     FilterConfig `yaml:"filters,omitempty" json:"filters,omitempty"`
	Sort        SortConfig   `yaml:"sort,omitempty" json:"sort,omitempty"`
	View        ViewConfig   `yaml:"view,omitempty" json:"view,omitempty"`
	Export      ExportConfig `yaml:"export,omitempty" json:"export,omitempty"`
	Metrics     []string     `yaml:"metrics,omitempty" json:"metrics,omitempty"` // Which metrics to show
}

// FilterConfig defines which issues to include
type FilterConfig struct {
	Status        []string `yaml:"status,omitempty" json:"status,omitempty"`                 // open, closed, in_progress, blocked
	Priority      []int    `yaml:"priority,omitempty" json:"priority,omitempty"`             // 0, 1, 2, 3
	Tags          []string `yaml:"tags,omitempty" json:"tags,omitempty"`                     // Include issues with these tags
	ExcludeTags   []string `yaml:"exclude_tags,omitempty" json:"exclude_tags,omitempty"`     // Exclude issues with these tags
	CreatedAfter  string   `yaml:"created_after,omitempty" json:"created_after,omitempty"`   // Relative: "14d", "1w", "2m" or ISO date
	CreatedBefore string   `yaml:"created_before,omitempty" json:"created_before,omitempty"` // Relative or ISO date
	UpdatedAfter  string   `yaml:"updated_after,omitempty" json:"updated_after,omitempty"`   // Relative or ISO date
	UpdatedBefore string   `yaml:"updated_before,omitempty" json:"updated_before,omitempty"` // Relative or ISO date
	HasBlockers   *bool    `yaml:"has_blockers,omitempty" json:"has_blockers,omitempty"`     // true = blocked, false = actionable
	Actionable    *bool    `yaml:"actionable,omitempty" json:"actionable,omitempty"`         // true = no open blockers
	TitleContains string   `yaml:"title_contains,omitempty" json:"title_contains,omitempty"` // Substring match
	IDPrefix      string   `yaml:"id_prefix,omitempty" json:"id_prefix,omitempty"`           // e.g., "bv-" for project filtering
}

// SortConfig defines how to order issues
type SortConfig struct {
	Field     string      `yaml:"field" json:"field"`                             // priority, created, updated, title, id, pagerank, betweenness
	Direction string      `yaml:"direction,omitempty" json:"direction,omitempty"` // asc, desc (default: asc for priority, desc for dates)
	Secondary *SortConfig `yaml:"secondary,omitempty" json:"secondary,omitempty"` // Tie-breaker
}

// ViewConfig controls display options
type ViewConfig struct {
	Columns       []string `yaml:"columns,omitempty" json:"columns,omitempty"`               // id, title, status, priority, created, updated, tags, blockers
	ShowGraph     bool     `yaml:"show_graph,omitempty" json:"show_graph,omitempty"`         // Show dependency graph in TUI
	ShowMetrics   bool     `yaml:"show_metrics,omitempty" json:"show_metrics,omitempty"`     // Show analysis metrics
	GroupBy       string   `yaml:"group_by,omitempty" json:"group_by,omitempty"`             // status, priority, tag, none
	Collapsed     bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`           // Start with groups collapsed
	MaxItems      int      `yaml:"max_items,omitempty" json:"max_items,omitempty"`           // Limit displayed items (0 = unlimited)
	TruncateTitle int      `yaml:"truncate_title,omitempty" json:"truncate_title,omitempty"` // Max title length
}

// ExportConfig controls output format options
type ExportConfig struct {
	Format       string `yaml:"format,omitempty" json:"format,omitempty"`               // markdown, json, csv, mermaid
	IncludeGraph bool   `yaml:"include_graph,omitempty" json:"include_graph,omitempty"` // Include Mermaid diagram
	Template     string `yaml:"template,omitempty" json:"template,omitempty"`           // Custom template path
}

// relativeTimePattern matches relative time expressions like "14d", "2w", "1m", "1y"
var relativeTimePattern = regexp.MustCompile(`^(\d+)([dwmy])$`)

// ParseRelativeTime converts a relative time string to an absolute time.
// Supports: Nd (days), Nw (weeks), Nm (months), Ny (years)
// If the string is not a relative time, it tries to parse as ISO 8601.
// Returns zero time if parsing fails.
func ParseRelativeTime(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	s = strings.TrimSpace(s)

	// Try relative time first (case-insensitive)
	if matches := relativeTimePattern.FindStringSubmatch(strings.ToLower(s)); matches != nil {
		n, _ := strconv.Atoi(matches[1])
		unit := matches[2]

		switch unit {
		case "d":
			return now.AddDate(0, 0, -n), nil
		case "w":
			return now.AddDate(0, 0, -n*7), nil
		case "m":
			return now.AddDate(0, -n, 0), nil
		case "y":
			return now.AddDate(-n, 0, 0), nil
		}
	}

	// Try ISO 8601 formats (preserve case for parsing)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, &TimeParseError{Input: s}
}

// TimeParseError indicates a time parsing failure
type TimeParseError struct {
	Input string
}

func (e *TimeParseError) Error() string {
	return "invalid time format: " + e.Input + " (expected relative like '14d', '2w', '1m' or ISO date)"
}

// DefaultRecipe returns a sensible default recipe
func DefaultRecipe() Recipe {
	return Recipe{
		Name:        "default",
		Description: "Default view showing all open issues sorted by priority",
		Filters: FilterConfig{
			Status: []string{"open", "in_progress", "blocked"},
		},
		Sort: SortConfig{
			Field:     "priority",
			Direction: "asc",
		},
		View: ViewConfig{
			Columns:   []string{"id", "title", "status", "priority"},
			ShowGraph: false,
			GroupBy:   "none",
		},
	}
}

// ActionableRecipe returns a recipe for actionable items only
func ActionableRecipe() Recipe {
	actionable := true
	return Recipe{
		Name:        "actionable",
		Description: "Issues ready to work on (no open blockers)",
		Filters: FilterConfig{
			Status:     []string{"open", "in_progress"},
			Actionable: &actionable,
		},
		Sort: SortConfig{
			Field:     "priority",
			Direction: "asc",
		},
		View: ViewConfig{
			Columns:     []string{"id", "title", "priority", "blockers"},
			ShowMetrics: true,
		},
	}
}

// RecentRecipe returns a recipe for recently updated issues
func RecentRecipe() Recipe {
	return Recipe{
		Name:        "recent",
		Description: "Issues updated in the last 7 days",
		Filters: FilterConfig{
			UpdatedAfter: "7d",
		},
		Sort: SortConfig{
			Field:     "updated",
			Direction: "desc",
		},
		View: ViewConfig{
			Columns: []string{"id", "title", "status", "updated"},
		},
	}
}

// BlockedRecipe returns a recipe showing blocked issues
func BlockedRecipe() Recipe {
	hasBlockers := true
	return Recipe{
		Name:        "blocked",
		Description: "Issues waiting on dependencies",
		Filters: FilterConfig{
			Status:      []string{"open", "in_progress", "blocked"},
			HasBlockers: &hasBlockers,
		},
		Sort: SortConfig{
			Field:     "priority",
			Direction: "asc",
		},
		View: ViewConfig{
			Columns:   []string{"id", "title", "priority", "blockers"},
			ShowGraph: true,
		},
	}
}

// HighImpactRecipe returns a recipe for high-impact issues (by PageRank)
func HighImpactRecipe() Recipe {
	return Recipe{
		Name:        "high-impact",
		Description: "Issues with highest blocking impact (PageRank)",
		Filters: FilterConfig{
			Status: []string{"open", "in_progress"},
		},
		Sort: SortConfig{
			Field:     "pagerank",
			Direction: "desc",
		},
		View: ViewConfig{
			Columns:     []string{"id", "title", "priority", "pagerank"},
			ShowMetrics: true,
			MaxItems:    20,
		},
		Metrics: []string{"pagerank", "critical_path"},
	}
}

// StaleRecipe returns a recipe for stale issues
func StaleRecipe() Recipe {
	return Recipe{
		Name:        "stale",
		Description: "Open issues not updated in 30+ days",
		Filters: FilterConfig{
			Status:        []string{"open", "in_progress"},
			UpdatedBefore: "30d",
		},
		Sort: SortConfig{
			Field:     "updated",
			Direction: "asc",
		},
		View: ViewConfig{
			Columns: []string{"id", "title", "status", "updated"},
		},
	}
}

// BuiltinRecipes returns all built-in recipes
func BuiltinRecipes() []Recipe {
	return []Recipe{
		DefaultRecipe(),
		ActionableRecipe(),
		RecentRecipe(),
		BlockedRecipe(),
		HighImpactRecipe(),
		StaleRecipe(),
	}
}

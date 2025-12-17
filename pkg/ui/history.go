// Package ui provides the history view for displaying bead-to-commit correlations.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/correlation"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// historyFocus tracks which pane has focus in the history view
type historyFocus int

const (
	historyFocusList historyFocus = iota
	historyFocusDetail
)

// historyViewMode tracks bead-centric vs git-centric view (bv-tl3n)
type historyViewMode int

const (
	historyModeBead historyViewMode = iota // Default: beads on left, commits for selected bead
	historyModeGit                         // Git mode: commits on left, related beads for selected commit
)

// CommitListEntry represents a commit in git-centric mode (bv-tl3n)
type CommitListEntry struct {
	SHA       string
	ShortSHA  string
	Message   string
	Author    string
	Timestamp string
	FileCount int
	BeadIDs   []string // Beads related to this commit
}

// historySearchMode tracks what type of search is active (bv-nkrj)
type historySearchMode int

const (
	searchModeOff    historySearchMode = iota // No search active
	searchModeAll                             // Search across all fields
	searchModeCommit                          // Search commit messages only
	searchModeSHA                             // Search by SHA prefix
	searchModeBead                            // Search bead ID/title
	searchModeAuthor                          // Search by author
)

// HistoryModel represents the TUI view for bead history and code correlations
type HistoryModel struct {
	// Data
	report    *correlation.HistoryReport
	histories []correlation.BeadHistory // Filtered and sorted list
	beadIDs   []string                  // Sorted bead IDs for navigation

	// Navigation state
	selectedBead   int // Index into beadIDs
	selectedCommit int // Index into selected bead's commits
	scrollOffset   int // For scrolling the bead list
	focused        historyFocus

	// Git-centric mode state (bv-tl3n)
	viewMode             historyViewMode
	commitList           []CommitListEntry // All commits sorted by recency
	selectedGitCommit    int               // Index into commitList
	selectedRelatedBead  int               // Index into selected commit's BeadIDs
	gitScrollOffset      int               // For scrolling the commit list

	// Filters
	authorFilter  string  // Filter by author (empty = all)
	minConfidence float64 // Minimum confidence threshold (0-1)

	// Search state (bv-nkrj)
	searchInput      textinput.Model   // Text input for search query
	searchMode       historySearchMode // Current search mode
	searchActive     bool              // Whether search input is focused
	lastSearchQuery  string            // Cache for detecting query changes
	filteredCommits  []CommitListEntry // Filtered commit list for git mode

	// Display state
	width  int
	height int
	theme  Theme

	// Expanded state tracking
	expandedBeads map[string]bool // Track which beads have commits expanded
}

// NewHistoryModel creates a new history view from a correlation report
func NewHistoryModel(report *correlation.HistoryReport, theme Theme) HistoryModel {
	// Initialize search input (bv-nkrj)
	ti := textinput.New()
	ti.Placeholder = "Search commits, beads, authors..."
	ti.CharLimit = 100
	ti.Width = 40

	h := HistoryModel{
		report:        report,
		theme:         theme,
		focused:       historyFocusList,
		minConfidence: 0.0, // Show all by default
		expandedBeads: make(map[string]bool),
		searchInput:   ti,
		searchMode:    searchModeOff,
	}
	h.rebuildFilteredList()
	return h
}

// SetReport updates the history data
func (h *HistoryModel) SetReport(report *correlation.HistoryReport) {
	h.report = report
	h.rebuildFilteredList()
}

// rebuildFilteredList rebuilds the filtered and sorted list of histories
func (h *HistoryModel) rebuildFilteredList() {
	// Capture current selection
	var selectedID string
	if h.selectedBead < len(h.beadIDs) {
		selectedID = h.beadIDs[h.selectedBead]
	}

	h.histories = nil
	h.beadIDs = nil

	if h.report == nil {
		return
	}

	// Filter and collect histories
	for beadID, history := range h.report.Histories {
		// Skip beads with no commits
		if len(history.Commits) == 0 {
			continue
		}

		// Apply author filter
		if h.authorFilter != "" {
			authorMatch := false
			for _, c := range history.Commits {
				if strings.Contains(strings.ToLower(c.Author), strings.ToLower(h.authorFilter)) ||
					strings.Contains(strings.ToLower(c.AuthorEmail), strings.ToLower(h.authorFilter)) {
					authorMatch = true
					break
				}
			}
			if !authorMatch {
				continue
			}
		}

		// Apply confidence filter - keep only commits meeting threshold
		if h.minConfidence > 0 {
			var filtered []correlation.CorrelatedCommit
			for _, c := range history.Commits {
				if c.Confidence >= h.minConfidence {
					filtered = append(filtered, c)
				}
			}
			if len(filtered) == 0 {
				continue
			}
			history.Commits = filtered
		}

		h.histories = append(h.histories, history)
		h.beadIDs = append(h.beadIDs, beadID)
	}

	// Sort by most commits first
	sort.Slice(h.histories, func(i, j int) bool {
		if len(h.histories[i].Commits) != len(h.histories[j].Commits) {
			return len(h.histories[i].Commits) > len(h.histories[j].Commits)
		}
		return h.histories[i].BeadID < h.histories[j].BeadID
	})

	// Rebuild beadIDs to match sorted order
	h.beadIDs = make([]string, len(h.histories))
	for i, hist := range h.histories {
		h.beadIDs[i] = hist.BeadID
	}

	// Restore selection if possible
	found := false
	if selectedID != "" {
		for i, id := range h.beadIDs {
			if id == selectedID {
				h.selectedBead = i
				found = true
				break
			}
		}
	}

	if found {
		// Clamp selected commit as commit list might have shrunk
		numCommits := len(h.histories[h.selectedBead].Commits)
		if h.selectedCommit >= numCommits {
			if numCommits > 0 {
				h.selectedCommit = numCommits - 1
			} else {
				h.selectedCommit = 0
			}
		}
	} else {
		// Reset selection if out of bounds or lost
		h.selectedBead = 0
		h.selectedCommit = 0
	}
}

// SetSize updates the view dimensions
func (h *HistoryModel) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetAuthorFilter sets the author filter and rebuilds the list
func (h *HistoryModel) SetAuthorFilter(author string) {
	h.authorFilter = author
	h.rebuildFilteredList()
}

// SetMinConfidence sets the minimum confidence threshold and rebuilds the list
func (h *HistoryModel) SetMinConfidence(conf float64) {
	h.minConfidence = conf
	h.rebuildFilteredList()
}

// Navigation methods

// MoveUp moves selection up in the current focus pane
func (h *HistoryModel) MoveUp() {
	if h.focused == historyFocusList {
		if h.selectedBead > 0 {
			h.selectedBead--
			h.selectedCommit = 0
			h.ensureBeadVisible()
		}
	} else {
		// In detail pane, move to previous commit
		if h.selectedCommit > 0 {
			h.selectedCommit--
		}
	}
}

// MoveDown moves selection down in the current focus pane
func (h *HistoryModel) MoveDown() {
	if h.focused == historyFocusList {
		if h.selectedBead < len(h.histories)-1 {
			h.selectedBead++
			h.selectedCommit = 0
			h.ensureBeadVisible()
		}
	} else {
		// In detail pane, move to next commit
		if h.selectedBead < len(h.histories) {
			commits := h.histories[h.selectedBead].Commits
			if h.selectedCommit < len(commits)-1 {
				h.selectedCommit++
			}
		}
	}
}

// ToggleFocus switches between list and detail panes
func (h *HistoryModel) ToggleFocus() {
	if h.focused == historyFocusList {
		h.focused = historyFocusDetail
	} else {
		h.focused = historyFocusList
	}
}

// NextCommit moves to the next commit within the selected bead (J key)
func (h *HistoryModel) NextCommit() {
	if h.selectedBead >= len(h.histories) {
		return
	}
	commits := h.histories[h.selectedBead].Commits
	if h.selectedCommit < len(commits)-1 {
		h.selectedCommit++
	}
}

// PrevCommit moves to the previous commit within the selected bead (K key)
func (h *HistoryModel) PrevCommit() {
	if h.selectedCommit > 0 {
		h.selectedCommit--
	}
}

// CycleConfidence cycles through common confidence thresholds (0, 0.5, 0.75, 0.9)
func (h *HistoryModel) CycleConfidence() {
	thresholds := []float64{0, 0.5, 0.75, 0.9}
	// Find current threshold index
	currentIdx := 0
	for i, t := range thresholds {
		if h.minConfidence >= t-0.01 && h.minConfidence <= t+0.01 {
			currentIdx = i
			break
		}
	}
	// Move to next threshold (wrap around)
	nextIdx := (currentIdx + 1) % len(thresholds)
	h.SetMinConfidence(thresholds[nextIdx])
}

// GetMinConfidence returns the current minimum confidence threshold
func (h *HistoryModel) GetMinConfidence() float64 {
	return h.minConfidence
}

// ToggleExpand expands/collapses the commits for the selected bead
func (h *HistoryModel) ToggleExpand() {
	if h.selectedBead < len(h.beadIDs) {
		beadID := h.beadIDs[h.selectedBead]
		h.expandedBeads[beadID] = !h.expandedBeads[beadID]
	}
}

// Search and Filter methods (bv-nkrj)

// StartSearch activates the search input
func (h *HistoryModel) StartSearch() {
	h.searchActive = true
	h.searchMode = searchModeAll
	h.searchInput.Focus()
}

// StartSearchWithMode activates search with a specific mode
func (h *HistoryModel) StartSearchWithMode(mode historySearchMode) {
	h.searchActive = true
	h.searchMode = mode
	h.searchInput.Focus()

	// Set appropriate placeholder based on mode
	switch mode {
	case searchModeCommit:
		h.searchInput.Placeholder = "Search commit messages..."
	case searchModeSHA:
		h.searchInput.Placeholder = "Enter SHA prefix..."
	case searchModeBead:
		h.searchInput.Placeholder = "Search bead ID or title..."
	case searchModeAuthor:
		h.searchInput.Placeholder = "Search by author..."
	default:
		h.searchInput.Placeholder = "Search commits, beads, authors..."
	}
}

// CancelSearch cancels the search and clears the query
func (h *HistoryModel) CancelSearch() {
	h.searchActive = false
	h.searchInput.Blur()
	h.searchInput.SetValue("")
	h.searchMode = searchModeOff
	h.lastSearchQuery = ""
	h.applySearchFilter()
}

// ClearSearch clears the search query but keeps search mode active
func (h *HistoryModel) ClearSearch() {
	h.searchInput.SetValue("")
	h.lastSearchQuery = ""
	h.applySearchFilter()
}

// IsSearchActive returns whether search input is active
func (h *HistoryModel) IsSearchActive() bool {
	return h.searchActive
}

// SearchQuery returns the current search query
func (h *HistoryModel) SearchQuery() string {
	return h.searchInput.Value()
}

// UpdateSearchInput updates the search input model (call from Update)
func (h *HistoryModel) UpdateSearchInput(msg interface{}) {
	h.searchInput, _ = h.searchInput.Update(msg)

	// Check if query changed and apply filter
	currentQuery := h.searchInput.Value()
	if currentQuery != h.lastSearchQuery {
		h.lastSearchQuery = currentQuery
		h.applySearchFilter()
	}
}

// applySearchFilter filters the data based on current search query
func (h *HistoryModel) applySearchFilter() {
	// Always rebuild base filtered list first (applies author/confidence filters)
	// This ensures we always filter from the complete set, not an already-filtered list
	// (bv-nkrj fix: backspacing to relax filter now works correctly)
	h.rebuildFilteredList()
	if h.viewMode == historyModeGit {
		h.buildCommitList()
	}

	query := strings.TrimSpace(h.searchInput.Value())
	if query == "" {
		h.filteredCommits = nil // Use full commitList in git mode
		return
	}

	// Apply search filter on top of base filters
	if h.viewMode == historyModeGit {
		h.filterCommitList(query)
	} else {
		h.filterBeadList(query)
	}
}

// filterCommitList filters commits in git mode based on search query
func (h *HistoryModel) filterCommitList(query string) {
	if len(h.commitList) == 0 {
		h.filteredCommits = nil
		return
	}

	query = strings.ToLower(query)
	var filtered []CommitListEntry

	for _, commit := range h.commitList {
		if h.commitMatchesQuery(commit, query) {
			filtered = append(filtered, commit)
		}
	}

	h.filteredCommits = filtered
	// Reset selection if out of bounds
	if h.selectedGitCommit >= len(filtered) {
		h.selectedGitCommit = 0
		h.selectedRelatedBead = 0
	}
	h.gitScrollOffset = 0
}

// commitMatchesQuery checks if a commit matches the search query
func (h *HistoryModel) commitMatchesQuery(commit CommitListEntry, query string) bool {
	switch h.searchMode {
	case searchModeSHA:
		return strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
			strings.HasPrefix(strings.ToLower(commit.ShortSHA), query)
	case searchModeCommit:
		return strings.Contains(strings.ToLower(commit.Message), query)
	case searchModeAuthor:
		return strings.Contains(strings.ToLower(commit.Author), query)
	case searchModeBead:
		for _, beadID := range commit.BeadIDs {
			if strings.Contains(strings.ToLower(beadID), query) {
				return true
			}
			// Also check bead title if available
			if h.report != nil {
				if hist, ok := h.report.Histories[beadID]; ok {
					if strings.Contains(strings.ToLower(hist.Title), query) {
						return true
					}
				}
			}
		}
		return false
	default: // searchModeAll - search across all fields
		if strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
			strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
			return true
		}
		if strings.Contains(strings.ToLower(commit.Message), query) {
			return true
		}
		if strings.Contains(strings.ToLower(commit.Author), query) {
			return true
		}
		for _, beadID := range commit.BeadIDs {
			if strings.Contains(strings.ToLower(beadID), query) {
				return true
			}
		}
		return false
	}
}

// filterBeadList filters beads in bead mode based on search query
func (h *HistoryModel) filterBeadList(query string) {
	if h.report == nil {
		return
	}

	query = strings.ToLower(query)
	var filteredHistories []correlation.BeadHistory
	var filteredIDs []string

	for _, beadID := range h.beadIDs {
		if hist, ok := h.report.Histories[beadID]; ok {
			if h.beadMatchesQuery(beadID, hist, query) {
				filteredHistories = append(filteredHistories, hist)
				filteredIDs = append(filteredIDs, beadID)
			}
		}
	}

	h.histories = filteredHistories
	h.beadIDs = filteredIDs

	// Reset selection if out of bounds
	if h.selectedBead >= len(h.beadIDs) {
		h.selectedBead = 0
		h.selectedCommit = 0
	}
	h.scrollOffset = 0
}

// beadMatchesQuery checks if a bead matches the search query
func (h *HistoryModel) beadMatchesQuery(beadID string, hist correlation.BeadHistory, query string) bool {
	switch h.searchMode {
	case searchModeBead:
		return strings.Contains(strings.ToLower(beadID), query) ||
			strings.Contains(strings.ToLower(hist.Title), query)
	case searchModeCommit:
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Message), query) {
				return true
			}
		}
		return false
	case searchModeSHA:
		for _, commit := range hist.Commits {
			if strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
				strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
				return true
			}
		}
		return false
	case searchModeAuthor:
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Author), query) {
				return true
			}
		}
		return false
	default: // searchModeAll
		// Check bead ID and title
		if strings.Contains(strings.ToLower(beadID), query) ||
			strings.Contains(strings.ToLower(hist.Title), query) {
			return true
		}
		// Check commits
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Message), query) ||
				strings.Contains(strings.ToLower(commit.Author), query) ||
				strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
				return true
			}
		}
		return false
	}
}

// GetFilteredCommitList returns the filtered commit list for git mode
func (h *HistoryModel) GetFilteredCommitList() []CommitListEntry {
	if h.filteredCommits != nil {
		return h.filteredCommits
	}
	return h.commitList
}

// GetSearchModeName returns a human-readable name for the current search mode
func (h *HistoryModel) GetSearchModeName() string {
	switch h.searchMode {
	case searchModeCommit:
		return "msg"
	case searchModeSHA:
		return "sha"
	case searchModeBead:
		return "bead"
	case searchModeAuthor:
		return "author"
	default:
		return "all"
	}
}

// Git-Centric View Mode methods (bv-tl3n)

// ToggleViewMode switches between Bead mode and Git mode
func (h *HistoryModel) ToggleViewMode() {
	if h.viewMode == historyModeBead {
		h.viewMode = historyModeGit
		h.buildCommitList()
		h.selectedGitCommit = 0
		h.selectedRelatedBead = 0
		h.gitScrollOffset = 0
	} else {
		h.viewMode = historyModeBead
		h.selectedBead = 0
		h.selectedCommit = 0
		h.scrollOffset = 0
	}
	// Re-apply search filter if active (bv-nkrj fix: filter persists across mode toggle)
	if h.searchActive && h.searchInput.Value() != "" {
		h.applySearchFilter()
	}
}

// IsGitMode returns true if in git-centric view mode
func (h *HistoryModel) IsGitMode() bool {
	return h.viewMode == historyModeGit
}

// buildCommitList constructs the sorted commit list for git mode
func (h *HistoryModel) buildCommitList() {
	if h.report == nil {
		h.commitList = nil
		return
	}

	seen := make(map[string]bool)
	var entries []CommitListEntry

	// Collect all commits from all bead histories
	for beadID, hist := range h.report.Histories {
		for _, commit := range hist.Commits {
			if seen[commit.SHA] {
				// Already have this commit, just add the bead ID
				for i := range entries {
					if entries[i].SHA == commit.SHA {
						// Check if bead already in list
						found := false
						for _, bid := range entries[i].BeadIDs {
							if bid == beadID {
								found = true
								break
							}
						}
						if !found {
							entries[i].BeadIDs = append(entries[i].BeadIDs, beadID)
						}
						break
					}
				}
				continue
			}
			seen[commit.SHA] = true

			entries = append(entries, CommitListEntry{
				SHA:       commit.SHA,
				ShortSHA:  commit.ShortSHA,
				Message:   commit.Message,
				Author:    commit.Author,
				Timestamp: commit.Timestamp.Format("2006-01-02 15:04"),
				FileCount: len(commit.Files),
				BeadIDs:   []string{beadID},
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	// Note: We parse from formatted string since we stored it that way
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	h.commitList = entries
}

// MoveUpGit moves selection up in git mode
func (h *HistoryModel) MoveUpGit() {
	if h.focused == historyFocusList {
		if h.selectedGitCommit > 0 {
			h.selectedGitCommit--
			h.selectedRelatedBead = 0
			h.ensureGitCommitVisible()
		}
	} else {
		// In detail pane, move to previous related bead
		if h.selectedRelatedBead > 0 {
			h.selectedRelatedBead--
		}
	}
}

// MoveDownGit moves selection down in git mode
func (h *HistoryModel) MoveDownGit() {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.focused == historyFocusList {
		if h.selectedGitCommit < len(commits)-1 {
			h.selectedGitCommit++
			h.selectedRelatedBead = 0
			h.ensureGitCommitVisible()
		}
	} else {
		// In detail pane, move to next related bead
		if h.selectedGitCommit < len(commits) {
			beadCount := len(commits[h.selectedGitCommit].BeadIDs)
			if h.selectedRelatedBead < beadCount-1 {
				h.selectedRelatedBead++
			}
		}
	}
}

// NextRelatedBead moves to the next related bead in git mode (J key)
func (h *HistoryModel) NextRelatedBead() {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.selectedGitCommit >= len(commits) {
		return
	}
	beadCount := len(commits[h.selectedGitCommit].BeadIDs)
	if h.selectedRelatedBead < beadCount-1 {
		h.selectedRelatedBead++
	}
}

// PrevRelatedBead moves to the previous related bead in git mode (K key)
func (h *HistoryModel) PrevRelatedBead() {
	if h.selectedRelatedBead > 0 {
		h.selectedRelatedBead--
	}
}

// SelectedGitCommit returns the selected commit in git mode
func (h *HistoryModel) SelectedGitCommit() *CommitListEntry {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.selectedGitCommit < len(commits) {
		return &commits[h.selectedGitCommit]
	}
	return nil
}

// SelectedRelatedBeadID returns the currently selected related bead ID in git mode
func (h *HistoryModel) SelectedRelatedBeadID() string {
	commit := h.SelectedGitCommit()
	if commit != nil && h.selectedRelatedBead < len(commit.BeadIDs) {
		return commit.BeadIDs[h.selectedRelatedBead]
	}
	return ""
}

// ensureGitCommitVisible adjusts scroll offset to keep selected commit visible
func (h *HistoryModel) ensureGitCommitVisible() {
	visibleItems := h.listHeight()
	if visibleItems < 1 {
		visibleItems = 1
	}

	if h.selectedGitCommit < h.gitScrollOffset {
		h.gitScrollOffset = h.selectedGitCommit
	} else if h.selectedGitCommit >= h.gitScrollOffset+visibleItems {
		h.gitScrollOffset = h.selectedGitCommit - visibleItems + 1
	}
}

// ensureBeadVisible adjusts scroll offset to keep selected bead visible
func (h *HistoryModel) ensureBeadVisible() {
	visibleItems := h.listHeight()
	if visibleItems < 1 {
		visibleItems = 1
	}

	if h.selectedBead < h.scrollOffset {
		h.scrollOffset = h.selectedBead
	} else if h.selectedBead >= h.scrollOffset+visibleItems {
		h.scrollOffset = h.selectedBead - visibleItems + 1
	}
}

// listHeight returns the number of visible items in the list
func (h *HistoryModel) listHeight() int {
	// Reserve 3 lines for header/filter bar
	return h.height - 3
}

// SelectedBeadID returns the currently selected bead ID
func (h *HistoryModel) SelectedBeadID() string {
	if h.selectedBead < len(h.beadIDs) {
		return h.beadIDs[h.selectedBead]
	}
	return ""
}

// SelectedHistory returns the currently selected bead history
func (h *HistoryModel) SelectedHistory() *correlation.BeadHistory {
	if h.selectedBead < len(h.histories) {
		return &h.histories[h.selectedBead]
	}
	return nil
}

// SelectedCommit returns the currently selected commit
func (h *HistoryModel) SelectedCommit() *correlation.CorrelatedCommit {
	hist := h.SelectedHistory()
	if hist != nil && h.selectedCommit < len(hist.Commits) {
		return &hist.Commits[h.selectedCommit]
	}
	return nil
}

// GetHistoryForBead returns the history for a specific bead ID
func (h *HistoryModel) GetHistoryForBead(beadID string) *correlation.BeadHistory {
	if h.report == nil {
		return nil
	}
	hist, ok := h.report.Histories[beadID]
	if !ok {
		return nil
	}
	return &hist
}

// HasReport returns true if history data is loaded
func (h *HistoryModel) HasReport() bool {
	return h.report != nil
}

// View renders the history view
func (h *HistoryModel) View() string {
	if h.report == nil {
		return h.renderEmpty("No history data loaded")
	}

	// In git mode, check commit list; in bead mode, check histories
	if h.viewMode == historyModeGit {
		if len(h.commitList) == 0 {
			return h.renderEmpty("No commits with bead correlations found")
		}
	} else {
		if len(h.histories) == 0 {
			return h.renderEmpty("No beads with commit correlations found")
		}
	}

	// Calculate panel widths (40% list, 60% detail)
	listWidth := int(float64(h.width) * 0.4)
	detailWidth := h.width - listWidth

	// Render header
	header := h.renderHeader()

	// Render panels based on view mode (bv-tl3n)
	var listPanel, detailPanel string
	if h.viewMode == historyModeGit {
		listPanel = h.renderGitCommitListPanel(listWidth, h.height-2)
		detailPanel = h.renderGitDetailPanel(detailWidth, h.height-2)
	} else {
		listPanel = h.renderListPanel(listWidth, h.height-2)
		detailPanel = h.renderDetailPanel(detailWidth, h.height-2)
	}

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels)
}

// renderEmpty renders an empty state message
func (h *HistoryModel) renderEmpty(msg string) string {
	t := h.theme
	style := t.Renderer.NewStyle().
		Width(h.width).
		Height(h.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(t.Secondary)

	return style.Render(msg + "\n\nPress H to close")
}

// renderHeader renders the filter bar and title
func (h *HistoryModel) renderHeader() string {
	t := h.theme

	titleStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Padding(0, 1)

	filterStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Padding(0, 1)

	// Show view mode indicator (bv-tl3n)
	var modeIndicator string
	if h.viewMode == historyModeGit {
		modeIndicator = "[Git Mode]"
	} else {
		modeIndicator = "[Bead Mode]"
	}
	modeStyle := t.Renderer.NewStyle().
		Foreground(t.InProgress).
		Bold(true).
		Padding(0, 1)

	title := titleStyle.Render("BEAD HISTORY") + modeStyle.Render(modeIndicator)

	// Build filter info
	var filters []string
	if h.viewMode == historyModeGit {
		// Show filtered count if search active (bv-nkrj)
		commits := h.GetFilteredCommitList()
		if h.searchActive && h.searchInput.Value() != "" {
			filters = append(filters, fmt.Sprintf("%d/%d commits", len(commits), len(h.commitList)))
		} else {
			filters = append(filters, fmt.Sprintf("%d commits", len(commits)))
		}
	} else {
		if h.searchActive && h.searchInput.Value() != "" {
			filters = append(filters, fmt.Sprintf("%d/%d beads", len(h.histories), len(h.report.Histories)))
		} else {
			filters = append(filters, fmt.Sprintf("%d/%d beads", len(h.histories), len(h.report.Histories)))
		}
	}

	if h.authorFilter != "" {
		filters = append(filters, fmt.Sprintf("Author: %s", h.authorFilter))
	}
	if h.minConfidence > 0 {
		filters = append(filters, fmt.Sprintf("Conf: ≥%.0f%%", h.minConfidence*100))
	}

	filterInfo := filterStyle.Render(strings.Join(filters, " | "))

	// Search input or close hint (bv-nkrj)
	var rightContent string
	if h.searchActive {
		// Show search input
		searchStyle := t.Renderer.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary).
			Padding(0, 1)

		modeLabel := t.Renderer.NewStyle().
			Foreground(t.Secondary).
			Render(fmt.Sprintf("[%s] ", h.GetSearchModeName()))

		inputView := h.searchInput.View()
		searchBox := searchStyle.Render(modeLabel + inputView)

		escHint := t.Renderer.NewStyle().
			Foreground(t.Muted).
			Padding(0, 1).
			Render("[Esc] cancel")

		rightContent = searchBox + escHint
	} else {
		// Show close hint and search hint
		rightContent = t.Renderer.NewStyle().
			Foreground(t.Muted).
			Padding(0, 1).
			Render("[/] search  [H] close")
	}

	// Combine with spacing
	spacerWidth := h.width - lipgloss.Width(title) - lipgloss.Width(filterInfo) - lipgloss.Width(rightContent)
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, title, filterInfo, spacer, rightContent)

	// Add separator line
	separatorWidth := h.width
	if separatorWidth < 1 {
		separatorWidth = 1
	}
	separator := t.Renderer.NewStyle().
		Foreground(t.Muted).
		Width(h.width).
		Render(strings.Repeat("─", separatorWidth))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, separator)
}

// renderListPanel renders the left panel with bead list
func (h *HistoryModel) renderListPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusList {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2). // Account for border
		Height(height - 2)

	// Column header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width - 4)
	header := headerStyle.Render("BEADS WITH HISTORY")

	// Build list content
	var lines []string
	lines = append(lines, header)
	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", sepWidth))

	visibleItems := height - 5 // Account for header, separator, border
	if visibleItems < 1 {
		visibleItems = 1
	}

	for i := h.scrollOffset; i < len(h.histories) && i < h.scrollOffset+visibleItems; i++ {
		hist := h.histories[i]
		line := h.renderBeadLine(i, hist, width-4)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderBeadLine renders a single bead in the list
func (h *HistoryModel) renderBeadLine(idx int, hist correlation.BeadHistory, width int) string {
	t := h.theme

	selected := idx == h.selectedBead

	// Indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// Status icon
	statusIcon := "○"
	switch hist.Status {
	case "closed":
		statusIcon = "✓"
	case "in_progress":
		statusIcon = "●"
	}

	// Commit count
	commitCount := fmt.Sprintf("%d commits", len(hist.Commits))

	// Truncate title
	maxTitleLen := width - len(indicator) - len(statusIcon) - len(commitCount) - 6
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := hist.Title
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-1] + "…"
	}

	// Build line
	idStyle := t.Renderer.NewStyle().Foreground(t.Secondary).Width(12)
	titleStyle := t.Renderer.NewStyle().Width(maxTitleLen)
	countStyle := t.Renderer.NewStyle().Foreground(t.Muted).Align(lipgloss.Right)

	if selected && h.focused == historyFocusList {
		idStyle = idStyle.Bold(true).Foreground(t.Primary)
		titleStyle = titleStyle.Bold(true)
	}

	line := fmt.Sprintf("%s%s %s %s %s",
		indicator,
		statusIcon,
		idStyle.Render(hist.BeadID),
		titleStyle.Render(title),
		countStyle.Render(commitCount),
	)

	return line
}

// renderDetailPanel renders the right panel with commit details
func (h *HistoryModel) renderDetailPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusDetail {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	hist := h.SelectedHistory()
	if hist == nil {
		return panelStyle.Render("No bead selected")
	}

	// Header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary)
	header := headerStyle.Render("COMMIT DETAILS")

	// Bead info
	beadInfo := fmt.Sprintf("%s: %s", hist.BeadID, hist.Title)
	if width > 10 && len(beadInfo) > width-6 {
		beadInfo = beadInfo[:width-7] + "…"
	} else if width <= 10 && len(beadInfo) > 5 {
		beadInfo = beadInfo[:4] + "…"
	}
	beadInfoStyle := t.Renderer.NewStyle().Foreground(t.Secondary)

	var lines []string
	lines = append(lines, header)
	lines = append(lines, beadInfoStyle.Render(beadInfo))
	detailSepWidth := width - 4
	if detailSepWidth < 1 {
		detailSepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// Render commits
	for i, commit := range hist.Commits {
		isSelected := i == h.selectedCommit && h.focused == historyFocusDetail
		commitLines := h.renderCommitDetail(commit, width-4, isSelected)
		lines = append(lines, commitLines...)
		if i < len(hist.Commits)-1 {
			lines = append(lines, "") // Spacer between commits
		}
	}

	// Pad with empty lines
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderCommitDetail renders details for a single commit
func (h *HistoryModel) renderCommitDetail(commit correlation.CorrelatedCommit, width int, selected bool) []string {
	t := h.theme

	var lines []string

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// SHA and message
	shaStyle := t.Renderer.NewStyle().Foreground(t.Primary)
	if selected {
		shaStyle = shaStyle.Bold(true)
	}
	shaLine := fmt.Sprintf("%s%s %s", indicator, shaStyle.Render(commit.ShortSHA), truncate(commit.Message, width-15))
	lines = append(lines, shaLine)

	// Author and date
	authorStyle := t.Renderer.NewStyle().Foreground(t.Secondary)
	authorLine := fmt.Sprintf("    %s • %s", authorStyle.Render(commit.Author), commit.Timestamp.Format("2006-01-02 15:04"))
	lines = append(lines, authorLine)

	// Confidence and method
	confStyle := t.Renderer.NewStyle()
	switch {
	case commit.Confidence >= 0.8:
		confStyle = confStyle.Foreground(t.Open) // Green
	case commit.Confidence >= 0.5:
		confStyle = confStyle.Foreground(t.Secondary) // Yellow/neutral
	default:
		confStyle = confStyle.Foreground(t.Muted) // Gray
	}

	methodStr := methodLabel(commit.Method)
	confLine := fmt.Sprintf("    %s %s",
		confStyle.Render(fmt.Sprintf("%.0f%%", commit.Confidence*100)),
		methodStr,
	)
	lines = append(lines, confLine)

	// Files (abbreviated)
	if len(commit.Files) > 0 {
		fileCount := fmt.Sprintf("    %d files changed", len(commit.Files))
		if len(commit.Files) <= 3 {
			var filenames []string
			for _, f := range commit.Files {
				filenames = append(filenames, f.Path)
			}
			fileCount = fmt.Sprintf("    %s", strings.Join(filenames, ", "))
			if width > 6 && len(fileCount) > width-2 {
				fileCount = fileCount[:width-3] + "…"
			} else if width <= 6 && len(fileCount) > 5 {
				fileCount = fileCount[:4] + "…"
			}
		}
		fileStyle := t.Renderer.NewStyle().Foreground(t.Muted)
		lines = append(lines, fileStyle.Render(fileCount))
	}

	return lines
}

// Helper functions



func methodLabel(method correlation.CorrelationMethod) string {
	switch method {
	case correlation.MethodCoCommitted:
		return "(co-committed)"
	case correlation.MethodExplicitID:
		return "(explicit ID)"
	case correlation.MethodTemporalAuthor:
		return "(temporal)"
	default:
		return ""
	}
}

// Git Mode rendering functions (bv-tl3n)

// renderGitCommitListPanel renders the left panel with commit list in git mode
func (h *HistoryModel) renderGitCommitListPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusList {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	// Column header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width - 4)
	header := headerStyle.Render("COMMITS")

	// Build list content
	var lines []string
	lines = append(lines, header)
	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", sepWidth))

	visibleItems := height - 5
	if visibleItems < 1 {
		visibleItems = 1
	}

	// Use filtered list if search is active (bv-nkrj)
	commits := h.GetFilteredCommitList()
	for i := h.gitScrollOffset; i < len(commits) && i < h.gitScrollOffset+visibleItems; i++ {
		commit := commits[i]
		line := h.renderGitCommitLine(i, commit, width-4)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderGitCommitLine renders a single commit in git mode list
func (h *HistoryModel) renderGitCommitLine(idx int, commit CommitListEntry, width int) string {
	t := h.theme

	selected := idx == h.selectedGitCommit

	// Indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// Bead count badge
	beadCount := fmt.Sprintf("[%d]", len(commit.BeadIDs))

	// Truncate message
	maxMsgLen := width - len(indicator) - len(commit.ShortSHA) - len(beadCount) - 6
	if maxMsgLen < 10 {
		maxMsgLen = 10
	}
	msg := commit.Message
	if len(msg) > maxMsgLen {
		msg = msg[:maxMsgLen-1] + "…"
	}

	// Build line
	shaStyle := t.Renderer.NewStyle().Foreground(t.Primary)
	msgStyle := t.Renderer.NewStyle()
	countStyle := t.Renderer.NewStyle().Foreground(t.Secondary)

	if selected && h.focused == historyFocusList {
		shaStyle = shaStyle.Bold(true)
		msgStyle = msgStyle.Bold(true)
	}

	line := fmt.Sprintf("%s%s %s %s",
		indicator,
		shaStyle.Render(commit.ShortSHA),
		msgStyle.Render(msg),
		countStyle.Render(beadCount),
	)

	return line
}

// renderGitDetailPanel renders the right panel with related beads and commit details in git mode
func (h *HistoryModel) renderGitDetailPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusDetail {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	commit := h.SelectedGitCommit()
	if commit == nil {
		return panelStyle.Render("No commit selected")
	}

	var lines []string

	// Header: Related Beads
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary)
	lines = append(lines, headerStyle.Render("RELATED BEADS"))

	detailSepWidth := width - 4
	if detailSepWidth < 1 {
		detailSepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// List related beads
	for i, beadID := range commit.BeadIDs {
		isSelected := i == h.selectedRelatedBead && h.focused == historyFocusDetail

		indicator := "  "
		if isSelected {
			indicator = "▸ "
		}

		// Get bead info from report
		beadStyle := t.Renderer.NewStyle()
		statusIcon := "○"
		title := beadID

		if h.report != nil {
			if hist, ok := h.report.Histories[beadID]; ok {
				title = hist.Title
				switch hist.Status {
				case "closed":
					statusIcon = "✓"
				case "in_progress":
					statusIcon = "●"
				}
			}
		}

		if isSelected {
			beadStyle = beadStyle.Bold(true).Foreground(t.Primary)
		}

		// Truncate title
		maxLen := width - 8
		if maxLen < 10 {
			maxLen = 10
		}
		if len(title) > maxLen {
			title = title[:maxLen-1] + "…"
		}

		beadLine := fmt.Sprintf("%s%s %s %s", indicator, statusIcon, beadID, beadStyle.Render(title))
		lines = append(lines, beadLine)
	}

	// Add separator before commit details
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("─", detailSepWidth))
	lines = append(lines, headerStyle.Render("COMMIT DETAILS"))
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// Commit details
	shaLine := fmt.Sprintf("SHA: %s", commit.SHA)
	if width > 10 && len(shaLine) > width-6 {
		shaLine = shaLine[:width-7] + "…"
	}
	lines = append(lines, t.Renderer.NewStyle().Foreground(t.Primary).Render(shaLine))

	authorLine := fmt.Sprintf("Author: %s", commit.Author)
	if width > 10 && len(authorLine) > width-6 {
		authorLine = authorLine[:width-7] + "…"
	}
	lines = append(lines, t.Renderer.NewStyle().Foreground(t.Secondary).Render(authorLine))

	dateLine := fmt.Sprintf("Date: %s", commit.Timestamp)
	lines = append(lines, t.Renderer.NewStyle().Foreground(t.Muted).Render(dateLine))

	filesLine := fmt.Sprintf("Files: %d changed", commit.FileCount)
	lines = append(lines, t.Renderer.NewStyle().Foreground(t.Muted).Render(filesLine))

	// Message
	lines = append(lines, "")
	msgStyle := t.Renderer.NewStyle().Foreground(t.Base.GetForeground())
	msgLines := strings.Split(commit.Message, "\n")
	for _, ml := range msgLines {
		if width > 6 && len(ml) > width-6 {
			ml = ml[:width-7] + "…"
		}
		lines = append(lines, msgStyle.Render(ml))
	}

	// Pad with empty lines
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

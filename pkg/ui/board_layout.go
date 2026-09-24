package ui

import (
	"sort"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// BoardLayoutKind selects how the board presents its columns. Both layouts
// render the same BoardModel state (columns, selection, search, swimlanes), so
// switching never loses the selected issue. See docs/adr/0016.
type BoardLayoutKind int

const (
	// BoardLayoutAdaptive is concept A: the focused column takes most of the
	// width, rows carry two lines of metadata, empty and closed columns fold
	// into rails.
	BoardLayoutAdaptive BoardLayoutKind = iota
	// BoardLayoutInspector is concept E: one-line rows beside a persistent
	// inspector for the selected issue.
	BoardLayoutInspector
)

func (k BoardLayoutKind) String() string {
	if k == BoardLayoutInspector {
		return "inspector"
	}
	return "adaptive"
}

// Label is the name the board bar and status line show.
func (k BoardLayoutKind) Label() string {
	if k == BoardLayoutInspector {
		return "E focus + inspector"
	}
	return "A adaptive focus"
}

func (k BoardLayoutKind) letter() string {
	if k == BoardLayoutInspector {
		return "E"
	}
	return "A"
}

func (k BoardLayoutKind) other() BoardLayoutKind {
	if k == BoardLayoutInspector {
		return BoardLayoutAdaptive
	}
	return BoardLayoutInspector
}

// ParseBoardLayout reads a ui.board_layout config value. The mockup letters
// are accepted beside the names because reviewers refer to the layouts by them.
func ParseBoardLayout(s string) (BoardLayoutKind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "adaptive", "a":
		return BoardLayoutAdaptive, true
	case "inspector", "e":
		return BoardLayoutInspector, true
	}
	return BoardLayoutAdaptive, false
}

// boardBreakpoints gives the minimum width at which 4, 3 and 2 columns are
// shown in full. Below the last one only the focused column is full.
type boardBreakpoints [3]int

var (
	// Spec widths from docs/board-redesign-spec.md "Responsive layout".
	adaptiveBreakpoints = boardBreakpoints{160, 110, 80}
	// One-line rows need less width, and the inspector already takes a share.
	inspectorBreakpoints = boardBreakpoints{110, 80, 56}
)

const (
	boardRailWidth    = 14
	boardMinRailWidth = 6
	boardMinFullWidth = 22
)

type boardRegionInput struct {
	col   int
	count int
	// preferRail marks a column that folds even when populated: closed work in
	// status mode is history, not a queue to act on.
	preferRail bool
}

type boardRegion struct {
	col       int
	width     int
	collapsed bool
}

// planBoardRegions decides which columns render in full, which fold into
// rails and how wide each one is. The result is in column order, never wider
// than width (separators included), and always contains the focused column.
func planBoardRegions(width int, inputs []boardRegionInput, focusedCol int, bp boardBreakpoints) []boardRegion {
	if len(inputs) == 0 || width <= 0 {
		return nil
	}
	focusPos := 0
	for i, in := range inputs {
		if in.col == focusedCol {
			focusPos = i
		}
	}

	budget := 1
	switch {
	case width >= bp[0]:
		budget = 4
	case width >= bp[1]:
		budget = 3
	case width >= bp[2]:
		budget = 2
	}

	// Candidates for a full region, nearest to the focus first.
	var candidates []int
	for i, in := range inputs {
		if i != focusPos && in.count > 0 && !in.preferRail {
			candidates = append(candidates, i)
		}
	}
	sort.SliceStable(candidates, func(a, b int) bool {
		return absInt(candidates[a]-focusPos) < absInt(candidates[b]-focusPos)
	})
	full := map[int]bool{focusPos: true}
	for _, i := range candidates {
		if len(full) >= budget {
			break
		}
		full[i] = true
	}

	shown := make([]bool, len(inputs))
	for i := range shown {
		shown[i] = true
	}

	for {
		nShown, nRails := 0, 0
		for i := range inputs {
			if shown[i] {
				nShown++
				if !full[i] {
					nRails++
				}
			}
		}
		fullW := width - (nShown - 1) - nRails*boardRailWidth
		if fullW >= boardMinFullWidth*len(full) {
			return layoutRegions(inputs, shown, full, focusPos, fullW, boardRailWidth)
		}
		// Drop full neighbours before squeezing rails: a rail still names the column.
		if len(full) > 1 {
			delete(full, farthest(full, focusPos))
			continue
		}
		railW := boardMinRailWidth
		if nRails > 0 {
			railW = (width - (nShown - 1) - boardMinFullWidth) / nRails
		}
		if nRails == 0 || railW >= boardMinRailWidth {
			if railW > boardRailWidth {
				railW = boardRailWidth
			}
			fullW = width - (nShown - 1) - nRails*railW
			return layoutRegions(inputs, shown, full, focusPos, fullW, railW)
		}
		// Not even narrow rails fit: hide the rail farthest from the focus.
		far, farDist := -1, -1
		for i := range inputs {
			if shown[i] && !full[i] && absInt(i-focusPos) > farDist {
				far, farDist = i, absInt(i-focusPos)
			}
		}
		shown[far] = false
	}
}

// layoutRegions splits fullW over the full regions, giving the focused column
// a 1.6 share so it reads as the working column.
func layoutRegions(inputs []boardRegionInput, shown []bool, full map[int]bool, focusPos, fullW, railW int) []boardRegion {
	shares := 0
	for i := range full {
		if i == focusPos {
			shares += 8
		} else {
			shares += 5
		}
	}
	var regions []boardRegion
	used := 0
	for i, in := range inputs {
		if !shown[i] {
			continue
		}
		r := boardRegion{col: in.col, collapsed: !full[i], width: railW}
		if full[i] && i != focusPos {
			r.width = fullW * 5 / shares
			used += r.width
		}
		regions = append(regions, r)
	}
	for i := range regions {
		if regions[i].col == inputs[focusPos].col {
			regions[i].width = fullW - used
		}
	}
	return regions
}

func farthest(set map[int]bool, from int) int {
	best, bestDist := -1, -1
	for i := range set {
		if i == from {
			continue
		}
		if d := absInt(i - from); d > bestDist || (d == bestDist && i > best) {
			best, bestDist = i, d
		}
	}
	return best
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// boardCardView is the presentation contract for one board row. It keeps the
// four facts the spec separates: stored status (the column), dependency
// readiness (BlockedBy), dispatcher lane state (LaneStage) and downstream
// impact (BlocksCount).
type boardCardView struct {
	ShortID     string
	Title       string
	Type        string
	Priority    string
	Age         string
	BlockedBy   string
	LaneStage   string
	BlocksCount int
}

// cardView derives the row fields for an issue. The issue is sanitized here so
// no caller can render a raw title, ID or label.
func (b *BoardModel) cardView(raw model.Issue) boardCardView {
	issue := sanitizeIssueForTerminal(raw)
	return boardCardView{
		ShortID:     b.displayID(issue.ID),
		Title:       withoutOwnIDPrefix(issue.Title, issue.ID),
		Type:        strings.ToLower(string(issue.IssueType)),
		Priority:    formatPriority(issue.Priority),
		Age:         compactAge(issue),
		BlockedBy:   b.displayID(sanitizeTerminalLine(b.openBlocker(raw))),
		LaneStage:   dispatcherLaneStage(issue.Labels),
		BlocksCount: len(b.blocksIndex[raw.ID]),
	}
}

// displayID drops the project prefix in single-project mode, where every ID
// shares it. All-projects mode keeps the prefix because it tells projects apart.
func (b *BoardModel) displayID(id string) string {
	if b.activeProjectName == "" || id == "" {
		return id
	}
	if i := strings.LastIndex(id, "-"); i >= 0 && i < len(id)-1 {
		return id[i+1:]
	}
	return id
}

// withoutOwnIDPrefix drops a leading "[<path>] " that repeats the issue's own
// ID path. Beads titles carry it by convention, and the row already shows the ID.
func withoutOwnIDPrefix(title, id string) string {
	path := id
	if i := strings.LastIndex(id, "-"); i >= 0 {
		path = id[i+1:]
	}
	return strings.TrimPrefix(title, "["+path+"] ")
}

// openBlocker returns the first blocking dependency that is not closed. A
// dependency on an unknown issue counts as open: the board cannot prove it done.
func (b *BoardModel) openBlocker(issue model.Issue) string {
	for _, dep := range issue.Dependencies {
		if dep == nil || !dep.Type.IsBlocking() {
			continue
		}
		if blocker, ok := b.issueMap[dep.DependsOnID]; ok && blocker != nil && isClosedLikeStatus(blocker.Status) {
			continue
		}
		return dep.DependsOnID
	}
	return ""
}

// compactAge is the time since the last update in at most four cells.
func compactAge(issue model.Issue) string {
	if issue.UpdatedAt.IsZero() {
		return "-"
	}
	rel := strings.TrimSuffix(FormatTimeRel(issue.UpdatedAt), " ago")
	if rel == "unknown" {
		return "-"
	}
	return rel
}

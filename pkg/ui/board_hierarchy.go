package ui

import "github.com/vanderheijden86/b9s/pkg/model"

// A board cell shows the parent-child hierarchy of its cards: a card sits
// directly below its parent and is indented one level further, so a feature
// and its tasks read as a group without a line of their own.
//
//	▲ mdu.1 Feature: baseline facts of the office network
//	  ✔ mdu.1.1 Record the Firebox model and Fireware version
//	  ✔ mdu.1.2 Record the contracted speed of the internet line
//
// Only a parent in the same cell counts. A card whose parent is in another
// column, or hidden by a filter, starts at the left edge like any other.

// cardLineIndent is the width of one hierarchy level on a one-line card.
const cardLineIndent = 2

// cardLineIndentMinWidth is the card width below which the hierarchy is not
// drawn: a narrow column needs every cell for the ID and the title.
const cardLineIndentMinWidth = cardLineTitleMin * 3

// cellParent returns the parent of id when the two share a cell, or "". An
// epic that heads a lane is the lane's header rather than a level, so its
// children start at the left edge.
func (b *BoardModel) cellParent(id string, inColumn map[string]bool) string {
	p := b.parentOf[id]
	if p == "" || !inColumn[p] || b.epicOf[p] == p || b.epicOf[p] != b.epicOf[id] {
		return ""
	}
	return p
}

// treeOrder returns one column's issues with every card directly below its
// parent, and records each card's depth. Cards without a parent in their cell
// keep the order they came in, and so do the children of one parent.
func (b *BoardModel) treeOrder(issues []model.Issue) []model.Issue {
	inColumn := make(map[string]bool, len(issues))
	for _, is := range issues {
		inColumn[is.ID] = true
	}
	children := make(map[string][]int)
	var roots []int
	for i, is := range issues {
		if p := b.cellParent(is.ID, inColumn); p != "" {
			children[p] = append(children[p], i)
		} else {
			roots = append(roots, i)
		}
	}
	if len(children) == 0 {
		return issues
	}

	ordered := make([]model.Issue, 0, len(issues))
	placed := make([]bool, len(issues))
	var place func(i, depth int)
	place = func(i, depth int) {
		if placed[i] {
			return
		}
		placed[i] = true
		ordered = append(ordered, issues[i])
		if depth > 0 {
			b.cardDepth[issues[i].ID] = depth
		}
		for _, c := range children[issues[i].ID] {
			place(c, depth+1)
		}
	}
	for _, i := range roots {
		place(i, 0)
	}
	// Imported data can carry a parent cycle (see maxEpicDepth). Its cards
	// have no root to hang from, and must stay on the board.
	for i := range issues {
		place(i, 0)
	}
	return ordered
}

// cardIndent is how far a one-line card of the given width is indented.
func (b *BoardModel) cardIndent(id string, width int) int {
	if width < cardLineIndentMinWidth {
		return 0
	}
	return min(b.cardDepth[id]*cardLineIndent, width/4)
}

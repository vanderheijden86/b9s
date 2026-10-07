package ui

import (
	"fmt"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// The detail pane numbers an issue's first nine children. With the detail
// focused, a digit opens that child and Backspace steps back through the
// issues opened this way, so a hierarchy can be walked without leaving it.

// detailChildKey opens the n-th child of the issue the detail shows.
func (m *Model) detailChildKey(n int) {
	shown := m.detailIssue()
	if shown == nil {
		return
	}
	ids := detailChildIDs(*shown, m.issueMap)
	if n > len(ids) {
		m.statusMsg, m.statusIsError = fmt.Sprintf("No child %d", n), false
		return
	}
	if !m.showInDetail(ids[n-1]) {
		m.statusMsg, m.statusIsError = fmt.Sprintf("Child %d is hidden by the filter", n), false
		return
	}
	m.detailBack = append(m.detailBack, shown.ID)
}

// detailBackKey returns to the issue a digit was pressed on. An issue that
// has since left the view is skipped.
func (m *Model) detailBackKey() {
	for len(m.detailBack) > 0 {
		id := m.detailBack[len(m.detailBack)-1]
		m.detailBack = m.detailBack[:len(m.detailBack)-1]
		if m.showInDetail(id) {
			return
		}
	}
}

// showInDetail makes id the issue the detail renders. In the tree the cursor
// is that selection, so it moves there and unfolds the ancestors on the way;
// the board and graph keep their cursor and only the detail changes.
func (m *Model) showInDetail(id string) bool {
	if m.treeViewActive && !m.isBoardView && !m.isGraphView {
		if _, ok := m.tree.ExpandPathTo(id); !ok || !m.tree.SelectByID(id) {
			return false
		}
		m.syncTreeToDetail()
		m.viewport.GotoTop()
		return true
	}
	for i, it := range m.list.Items() {
		if issueItem, ok := it.(IssueItem); ok && issueItem.Issue.ID == id {
			m.list.Select(i)
			m.detailPin = detailPin{}
			m.updateViewportContent()
			m.viewport.GotoTop()
			return true
		}
	}
	return false
}

// detailSiblingNavAvailable reports whether n and p step through sibling
// issues on the detail screen. Only the tree has siblings; the board and the
// graph keep their own cursor.
func (m Model) detailSiblingNavAvailable() bool {
	return m.treeViewActive && !m.isBoardView && !m.isGraphView
}

// The tree and the board can show issues the filtered list does not hold: a
// search keeps a matching branch whole, children included. A detailPin lets
// the detail show such an issue. It holds only while the list selection is
// still the one it was set against, so moving in the list takes the detail
// back to the list.
type detailPin struct {
	id         string
	listAnchor string
}

func (m *Model) listSelectedID() string {
	if item, ok := m.list.SelectedItem().(IssueItem); ok {
		return item.Issue.ID
	}
	return ""
}

// selectDetailIssue makes id the issue the detail shows, through the list
// when the list holds it and through the pin otherwise.
func (m *Model) selectDetailIssue(id string) {
	m.detailPin = detailPin{}
	for i, item := range m.list.Items() {
		if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == id {
			m.list.Select(i)
			return
		}
	}
	if _, ok := m.issueMap[id]; ok {
		m.detailPin = detailPin{id: id, listAnchor: m.listSelectedID()}
	}
}

// detailIssue returns the issue the detail shows, or nil when there is none.
func (m *Model) detailIssue() *model.Issue {
	if pin := m.detailPin; pin.id != "" && pin.listAnchor == m.listSelectedID() {
		if issue, ok := m.issueMap[pin.id]; ok {
			return issue
		}
	}
	if item, ok := m.list.SelectedItem().(IssueItem); ok {
		return &item.Issue
	}
	return nil
}

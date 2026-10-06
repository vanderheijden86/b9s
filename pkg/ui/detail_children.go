package ui

import "fmt"

// The detail pane numbers an issue's first nine children. With the detail
// focused, a digit opens that child and Backspace steps back through the
// issues opened this way, so a hierarchy can be walked without leaving it.

// detailChildKey opens the n-th child of the issue the detail shows.
func (m *Model) detailChildKey(n int) {
	item, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		return
	}
	ids := detailChildIDs(item.Issue, m.issueMap)
	if n > len(ids) {
		m.statusMsg, m.statusIsError = fmt.Sprintf("No child %d", n), false
		return
	}
	if !m.showInDetail(ids[n-1]) {
		m.statusMsg, m.statusIsError = fmt.Sprintf("Child %d is hidden by the filter", n), false
		return
	}
	m.detailBack = append(m.detailBack, item.Issue.ID)
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

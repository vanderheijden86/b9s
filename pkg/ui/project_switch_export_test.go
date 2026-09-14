package ui

// CompleteProjectSwitchForTest delivers a successful open for the switch in
// progress, so tests outside the package can observe the switched model
// without running the loader or waiting for the deadline.
func (m Model) CompleteProjectSwitchForTest() Model {
	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: m.projectSwitch.project})
	return updated.(Model)
}

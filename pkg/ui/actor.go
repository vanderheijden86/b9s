package ui

import (
	"github.com/vanderheijden86/beadwork/pkg/identity"
	"os"
	"os/exec"
	"strings"
)

// actorLookup finds the bd actor for writes made from b9s. The environment
// and git are injected so tests do not depend on the machine running them.
type actorLookup struct {
	getenv      func(string) string
	gitUserName func(dir string) string
}

var defaultActorLookup = actorLookup{
	getenv: os.Getenv,
	gitUserName: func(dir string) string {
		cmd := exec.Command("git", "config", "user.name")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	},
}

// actor follows bd's getActorWithGit order: BEADS_ACTOR, BD_ACTOR, git
// user.name in the project, USER. Where bd falls back to "unknown" this
// returns "", so b9s prefills nothing rather than a placeholder.
func (l actorLookup) actor(projectDir string) string {
	if l.getenv == nil {
		l = defaultActorLookup
	}
	for _, key := range []string{"BEADS_ACTOR", "BD_ACTOR"} {
		if v := l.getenv(key); v != "" {
			return v
		}
	}
	if projectDir != "" && l.gitUserName != nil {
		if v := l.gitUserName(projectDir); v != "" {
			return v
		}
	}
	return l.getenv("USER")
}

// currentActor is the actor resolved through the identity registry, so the
// creator b9s records is the name the project's people list uses.
func (m Model) currentActor() string {
	return m.currentIdentity().Name
}

// currentIdentity resolves the actor through b9s.identities.
func (m Model) currentIdentity() identity.Identity {
	// bd reads git user.name in the checkout it runs in, so the lookup does too.
	dir := m.issueWriter.Dir()
	if dir == "" {
		dir = m.activeProjectPath
	}
	return m.identities.Resolve(m.actorLookup.actor(dir))
}

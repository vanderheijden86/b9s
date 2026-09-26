// Package control lets another process on the same machine steer a running
// b9s. Each TUI instance listens on a unix socket in a private state
// directory and registers itself, with its tmux pane, beside the socket.
// `b9s ctl` finds an instance there and sends it one request per connection.
//
// The protocol accepts a closed set of verbs rather than key presses, so a
// request means the same thing whatever view, prompt or form b9s is showing.
package control

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Verb names one action a client may ask for.
type Verb string

// VerbBranch selects an issue and limits the view to its top-level branch.
const VerbBranch Verb = "branch"

var knownVerbs = map[Verb]bool{VerbBranch: true}

// Request is one line of JSON sent by a client.
type Request struct {
	Verb Verb   `json:"verb"`
	ID   string `json:"id,omitempty"`
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Instance is the registration a running b9s writes beside its socket.
type Instance struct {
	PID      int    `json:"pid"`
	Socket   string `json:"socket"`
	Cwd      string `json:"cwd,omitempty"`
	TmuxPane string `json:"tmux_pane,omitempty"`
}

const (
	ioTimeout  = 2 * time.Second
	maxRequest = 4096
)

// DefaultDir is where instances register: B9S_CONTROL_DIR when set, else
// $XDG_STATE_HOME/b9s/instances, else ~/.local/state/b9s/instances.
func DefaultDir() (string, error) {
	if dir := os.Getenv("B9S_CONTROL_DIR"); dir != "" {
		return dir, nil
	}
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "b9s", "instances"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "b9s", "instances"), nil
}

// Server is a listening instance. Close removes its socket and registration.
type Server struct {
	ln       net.Listener
	regPath  string
	sockPath string
}

// Serve registers inst in dir and calls handle for every valid request.
// handle runs on the connection's goroutine and must not block for long.
func Serve(dir string, inst Instance, handle func(Request) error) (*Server, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// The directory is the access boundary: anyone who can reach the socket
	// can steer b9s, so it must not stay wider than its owner.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	base := strconv.Itoa(inst.PID)
	sockPath := filepath.Join(dir, base+".sock")
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}
	inst.Socket = sockPath
	data, err := json.Marshal(inst)
	if err != nil {
		ln.Close()
		return nil, err
	}
	regPath := filepath.Join(dir, base+".json")
	if err := os.WriteFile(regPath, data, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{ln: ln, regPath: regPath, sockPath: sockPath}
	go s.accept(handle)
	return s, nil
}

func (s *Server) accept(handle func(Request) error) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go serveConn(conn, handle)
	}
}

func serveConn(conn net.Conn, handle func(Request) error) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	reply := response{OK: true}
	var req Request
	line, err := bufio.NewReader(io.LimitReader(conn, maxRequest)).ReadBytes('\n')
	switch {
	case err != nil && len(line) == 0:
		reply = response{Error: "no request"}
	case json.Unmarshal(line, &req) != nil:
		reply = response{Error: "request is not JSON"}
	case !knownVerbs[req.Verb]:
		reply = response{Error: fmt.Sprintf("unknown verb %q", req.Verb)}
	default:
		if err := handle(req); err != nil {
			reply = response{Error: err.Error()}
		}
	}
	_ = json.NewEncoder(conn).Encode(reply)
}

// Close stops listening and removes the socket and registration.
func (s *Server) Close() error {
	err := s.ln.Close()
	_ = os.Remove(s.sockPath)
	_ = os.Remove(s.regPath)
	return err
}

// List returns the registered instances whose process is alive, removing the
// registrations that a crashed b9s left behind.
func List(dir string) ([]Instance, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []Instance
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var inst Instance
		if json.Unmarshal(data, &inst) != nil || inst.PID <= 0 {
			continue
		}
		if !alive(inst.PID) {
			_ = os.Remove(path)
			_ = os.Remove(inst.Socket)
			continue
		}
		out = append(out, inst)
	}
	return out, nil
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Selector says which instance a client means.
type Selector struct {
	Pane       string // explicit tmux pane id, as given by --pane
	CallerPane string // the client's own $TMUX_PANE
}

// Pick chooses one instance. windows maps tmux pane ids to window ids. An
// explicit pane wins; inside tmux only an instance in the caller's window
// qualifies, because a b9s in another window shows someone else's work;
// outside tmux only a single running instance qualifies.
func Pick(instances []Instance, windows map[string]string, sel Selector) (Instance, error) {
	if sel.Pane != "" {
		for _, inst := range instances {
			if inst.TmuxPane == sel.Pane {
				return inst, nil
			}
		}
		return Instance{}, fmt.Errorf("no b9s in pane %s", sel.Pane)
	}
	if sel.CallerPane != "" {
		window := windows[sel.CallerPane]
		var match []Instance
		for _, inst := range instances {
			if inst.TmuxPane != "" && window != "" && windows[inst.TmuxPane] == window {
				match = append(match, inst)
			}
		}
		switch len(match) {
		case 1:
			return match[0], nil
		case 0:
			return Instance{}, errors.New("no b9s in this tmux window")
		default:
			return Instance{}, fmt.Errorf("%d b9s instances in this tmux window: pass --pane %s", len(match), panes(match))
		}
	}
	switch len(instances) {
	case 1:
		return instances[0], nil
	case 0:
		return Instance{}, errors.New("no b9s is running")
	default:
		return Instance{}, fmt.Errorf("%d b9s instances are running: pass --pane %s", len(instances), panes(instances))
	}
}

func panes(instances []Instance) string {
	var names []string
	for _, inst := range instances {
		if inst.TmuxPane != "" {
			names = append(names, inst.TmuxPane)
		}
	}
	if len(names) == 0 {
		return "<pane>"
	}
	return strings.Join(names, " or ")
}

// Send delivers req to inst and returns the instance's error, if any.
func Send(inst Instance, req Request) error {
	conn, err := net.DialTimeout("unix", inst.Socket, ioTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var reply response
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return fmt.Errorf("no reply from b9s: %w", err)
	}
	if !reply.OK {
		return errors.New(reply.Error)
	}
	return nil
}

// TmuxWindows maps every tmux pane id to its window id. It returns nil when
// tmux is not running, which Pick treats as no window matching.
func TmuxWindows() map[string]string {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id} #{window_id}").Output()
	if err != nil {
		return nil
	}
	return parseTmuxPanes(string(out))
}

func parseTmuxPanes(out string) map[string]string {
	windows := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if pane, window, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			windows[pane] = window
		}
	}
	return windows
}

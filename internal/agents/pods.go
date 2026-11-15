package agents

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// Omnigent lane labels.
const (
	LabelLaneStage   = "omnigent.ai/lane-stage"
	LabelLaneProject = "omnigent.ai/lane-project"
	LabelLaneTask    = "omnigent.ai/beads-task-id"
)

// PodSource says where to look for lane pods. Kubectl runs read-only verbs
// only (get, logs).
type PodSource struct {
	Context   string
	Namespace string // empty means all namespaces
	Selector  string // label selector; empty means LabelLaneStage
}

func (p PodSource) args(verb ...string) []string {
	a := append([]string{}, verb...)
	if p.Context != "" {
		a = append(a, "--context", p.Context)
	}
	return a
}

// ListPods returns one session per lane pod. A kubectl failure returns the
// error so the UI can show why the pods are missing.
func ListPods(ctx context.Context, src PodSource) ([]Session, error) {
	sel := src.Selector
	if sel == "" {
		sel = LabelLaneStage
	}
	args := src.args("get", "pods", "-l", sel, "-o", "json")
	if src.Namespace == "" {
		args = append(args, "-A")
	} else {
		args = append(args, "-n", src.Namespace)
	}
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, err
	}
	return ParsePods(out, time.Now())
}

// ParsePods reads `kubectl get pods -o json`.
func ParsePods(data []byte, now time.Time) ([]Session, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				Labels            map[string]string `json:"labels"`
				CreationTimestamp string            `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	var out []Session
	for _, it := range list.Items {
		m := it.Metadata
		started := parseTime(m.CreationTimestamp)
		s := Session{
			ID:           m.Namespace + "/" + m.Name,
			Name:         m.Name,
			Tool:         ToolLane,
			Project:      m.Labels[LabelLaneProject],
			Role:         m.Labels[LabelLaneStage],
			TaskID:       m.Labels[LabelLaneTask],
			Started:      started,
			LastActivity: started,
			Where:        Where{Kind: WherePod, Ref: m.Namespace + "/" + m.Name, Label: m.Name},
		}
		switch it.Status.Phase {
		case "Pending":
			s.State = StatePending
		case "Running":
			s.State = StateRunning
			s.LastActivity = now
		case "Failed":
			s.State = StateFailed
		default:
			s.State = StateDone
		}
		out = append(out, s)
	}
	return out, nil
}

// PodLogs reads the tail of a pod's log as events.
func PodLogs(ctx context.Context, src PodSource, ref string, tail int) ([]Event, error) {
	ns, name, ok := strings.Cut(ref, "/")
	if !ok {
		return nil, nil
	}
	args := src.args("logs", "-n", ns, name, "--all-containers", "--timestamps", "--tail", itoa(tail))
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, err
	}
	return ParsePodLog(string(out)), nil
}

// ParsePodLog turns `kubectl logs --timestamps` output into events.
func ParsePodLog(out string) []Event {
	var evs []Event
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if ln == "" {
			continue
		}
		ts, text, _ := strings.Cut(ln, " ")
		at := parseTime(ts)
		if at.IsZero() {
			text = ln
		}
		kind := EventAI
		low := strings.ToLower(text)
		switch {
		case strings.Contains(low, "error") || strings.Contains(low, "fail"):
			kind = EventErr
		case bdCmdRe.MatchString(" " + text):
			kind = EventBD
		}
		evs = append(evs, Event{Time: at, Kind: kind, Text: oneLine(text, 400), Detail: text})
	}
	return evs
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

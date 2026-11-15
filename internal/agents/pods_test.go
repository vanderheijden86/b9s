package agents

import (
	"testing"
	"time"
)

func TestParsePods(t *testing.T) {
	data := []byte(`{"items":[{"metadata":{"name":"lane-abc","namespace":"lanes","creationTimestamp":"2026-11-15T08:00:00Z","labels":{"omnigent.ai/lane-stage":"implement","omnigent.ai/lane-project":"b9s","omnigent.ai/beads-task-id":"bd-ab12"}},"status":{"phase":"Running"}},{"metadata":{"name":"lane-def","namespace":"lanes","labels":{}},"status":{"phase":"Pending"}}]}`)
	got, err := ParsePods(data, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Project != "b9s" || got[0].Role != "implement" || got[0].TaskID != "bd-ab12" || got[0].State != StateRunning || got[0].Where.Kind != WherePod {
		t.Fatalf("pods = %+v", got)
	}
	if got[1].State != StatePending {
		t.Errorf("pending state = %s", got[1].State)
	}
}

func TestParsePodLog(t *testing.T) {
	ev := ParsePodLog("2026-11-15T08:00:00Z starting\n2026-11-15T08:00:01Z error: boom\n")
	if len(ev) != 2 || ev[1].Kind != EventErr || ev[0].Time.IsZero() {
		t.Fatalf("events = %+v", ev)
	}
}

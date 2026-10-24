package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Aasthik17/ctrlz/internal/store"
)

func fixtureModel() model {
	data := historyData{snapshots: []store.Snapshot{
		{Hash: "aaaaaaaaaaaa1111111111111111111111111111", Timestamp: time.Now(), Reason: store.ReasonInterval, Stat: "1 file changed"},
		{Hash: "bbbbbbbbbbbb2222222222222222222222222222", Timestamp: time.Now(), Reason: store.ReasonPreUndo, Stat: "2 files changed"},
	}}
	m := newModel(&store.Project{ID: "test", Path: "/tmp/project"}, "/tmp/store", data)
	m.width, m.height = 120, 35
	return m
}

func TestSelectionIgnoresStaleDiff(t *testing.T) {
	m := fixtureModel()
	m.diffReq = 3
	updated, _ := m.Update(diffMsg{request: 2, diff: "stale"})
	got := updated.(model)
	if got.diff != "" {
		t.Fatalf("stale diff replaced current content: %q", got.diff)
	}
	updated, _ = got.Update(diffMsg{request: 3, diff: "current"})
	if got = updated.(model); got.diff != "current" {
		t.Fatalf("matching diff was not applied: %q", got.diff)
	}
}

func TestNarrowTerminalHasResizeMessage(t *testing.T) {
	m := fixtureModel()
	m.width, m.height = 60, 16
	if got := m.View().Content; !strings.Contains(got, "Resize this terminal") {
		t.Fatalf("narrow layout did not explain the requirement: %q", got)
	}
}

func TestSelectionMaintainsVisibleTimeline(t *testing.T) {
	m := fixtureModel()
	m.snapshots = make([]store.Snapshot, 20)
	for i := range m.snapshots {
		m.snapshots[i].Hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	m.height = 18
	m.selectSnapshot(15)
	if m.selected != 15 || m.listTop == 0 {
		t.Fatalf("selection was not scrolled into view: selected=%d top=%d", m.selected, m.listTop)
	}
}

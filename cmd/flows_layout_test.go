package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowsLayoutWriteRearrangesFileOffline(t *testing.T) {
	// Two cards on top of each other; no Homey is configured, so any API
	// call would fail the test.
	file := filepath.Join(t.TempDir(), "draft.json")
	draft := `{"name":"Draft","cards":{
		"11111111-1111-4111-8111-111111111111":{"type":"trigger","id":"homey:manager:flow:programmatic_trigger","x":0,"y":0,"outputSuccess":["22222222-2222-4222-8222-222222222222"]},
		"22222222-2222-4222-8222-222222222222":{"type":"action","id":"homey:manager:notifications:create_notification","args":{"text":"hi"},"x":10,"y":10}}}`
	if err := os.WriteFile(file, []byte(draft), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = flowsLayoutCmd.Flags().Set("write", "false") })
	_ = flowsLayoutCmd.Flags().Set("write", "true")
	captureStdout(t, func() {
		if err := flowsLayoutCmd.RunE(flowsLayoutCmd, []string{file}); err != nil {
			t.Fatal(err)
		}
	})

	var doc map[string]any
	data, _ := os.ReadFile(file)
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	cards := doc["cards"].(map[string]any)
	a := cards["11111111-1111-4111-8111-111111111111"].(map[string]any)
	b := cards["22222222-2222-4222-8222-222222222222"].(map[string]any)
	if b["x"].(float64)-a["x"].(float64) < 400 {
		t.Fatalf("cards still crowded: a.x=%v b.x=%v", a["x"], b["x"])
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode changed to %v", info.Mode().Perm())
	}
}

func TestFlowsLayoutRejectsSimpleFlow(t *testing.T) {
	file := filepath.Join(t.TempDir(), "simple.json")
	_ = os.WriteFile(file, []byte(`{"name":"Simple","trigger":{"id":"homey:manager:flow:programmatic_trigger"},"actions":[]}`), 0o600)
	err := flowsLayoutCmd.RunE(flowsLayoutCmd, []string{file})
	if err == nil || !strings.Contains(err.Error(), "only Advanced Flows") {
		t.Fatalf("expected an Advanced-Flow-only error, got %v", err)
	}
}

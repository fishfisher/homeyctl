package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

const (
	refVarUsed   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	refVarUnused = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	refVarScript = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

// refFixture: one flow uses AI.Used via a droptoken, a HomeyScript looks up
// AI.ScriptOnly by name, and AI.Unused is referenced nowhere.
func refFixture(t *testing.T) (deletes *int) {
	t.Helper()
	deletes = new(int)
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/manager/logic/variable/":
			_, _ = w.Write([]byte(`{
				"` + refVarUsed + `":{"id":"` + refVarUsed + `","name":"AI.Used","type":"number","value":1},
				"` + refVarUnused + `":{"id":"` + refVarUnused + `","name":"AI.Unused","type":"number","value":0},
				"` + refVarScript + `":{"id":"` + refVarScript + `","name":"AI.ScriptOnly","type":"boolean","value":false}}`))
		case "DELETE /api/manager/logic/variable/" + refVarUsed, "DELETE /api/manager/logic/variable/" + refVarUnused, "DELETE /api/manager/logic/variable/" + refVarScript:
			*deletes++
			w.WriteHeader(200)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	old := loadUsageIndexFunc
	loadUsageIndexFunc = func() (*usageIndex, error) {
		return &usageIndex{
			flows: []indexedFlow{{id: "f1", name: "Heating", advanced: false, enabled: true, document: map[string]any{
				"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
				"conditions": []any{map[string]any{"id": "homey:manager:logic:lt", "droptoken": "homey:manager:logic|" + refVarUsed}},
				"actions":    []any{},
			}}},
			scripts: []indexedScript{{id: "s1", name: "Report", code: `const v = vars.find(v => v.name === "AI.ScriptOnly")`}},
		}, nil
	}
	oldBackup := backupFlow
	backupFlow = func(string, json.RawMessage) (string, error) { return "/test/backup.json", nil }
	t.Cleanup(func() {
		loadUsageIndexFunc, backupFlow = old, oldBackup
		for _, f := range []string{"force", "allow-referenced"} {
			_ = varsDeleteCmd.Flags().Set(f, "false")
		}
		_ = varsUsageCmd.Flags().Set("prefix", "")
		_ = varsUsageCmd.Flags().Set("unused", "false")
	})
	return deletes
}

func TestVariableUsageUnusedFindsOnlyTheUnreferenced(t *testing.T) {
	refFixture(t)
	oldJSON := jsonFlag
	jsonFlag = true
	t.Cleanup(func() { jsonFlag = oldJSON })
	_ = varsUsageCmd.Flags().Set("prefix", "ai.")
	_ = varsUsageCmd.Flags().Set("unused", "true")
	out := captureStdout(t, func() {
		if err := varsUsageCmd.RunE(varsUsageCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	var got struct{ Variables []VariableUsage }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", out, err)
	}
	// AI.ScriptOnly is only mentioned by name in a script: that counts as used,
	// since deleting it would break the script.
	if len(got.Variables) != 1 || got.Variables[0].Name != "AI.Unused" {
		t.Fatalf("unused = %+v, want only AI.Unused", got.Variables)
	}
}

func TestDeleteRefusesReferencedVariableBeforeAnyWrite(t *testing.T) {
	deletes := refFixture(t)
	backedUp := false
	backupFlow = func(string, json.RawMessage) (string, error) { backedUp = true; return "/test/backup.json", nil }
	_ = varsDeleteCmd.Flags().Set("force", "true")
	for _, name := range []string{"AI.Used", "AI.ScriptOnly"} {
		err := varsDeleteCmd.RunE(varsDeleteCmd, []string{name})
		if err == nil || !strings.Contains(err.Error(), "--allow-referenced") {
			t.Fatalf("%s: expected refusal, got %v", name, err)
		}
	}
	if *deletes != 0 || backedUp {
		t.Fatalf("refused delete still wrote: deletes=%d backup=%v", *deletes, backedUp)
	}
}

func TestDeleteAllowsUnusedAndOverride(t *testing.T) {
	deletes := refFixture(t)
	_ = varsDeleteCmd.Flags().Set("force", "true")
	captureStdout(t, func() {
		if err := varsDeleteCmd.RunE(varsDeleteCmd, []string{"AI.Unused"}); err != nil {
			t.Fatal(err)
		}
	})
	_ = varsDeleteCmd.Flags().Set("allow-referenced", "true")
	captureStdout(t, func() {
		if err := varsDeleteCmd.RunE(varsDeleteCmd, []string{"AI.Used"}); err != nil {
			t.Fatal(err)
		}
	})
	if *deletes != 2 {
		t.Fatalf("deletes = %d, want 2", *deletes)
	}
}

func TestDeleteRefusesWhenReferencesCannotBeChecked(t *testing.T) {
	deletes := refFixture(t)
	loadUsageIndexFunc = func() (*usageIndex, error) { return nil, errors.New("homey unreachable") }
	_ = varsDeleteCmd.Flags().Set("force", "true")
	err := varsDeleteCmd.RunE(varsDeleteCmd, []string{"AI.Unused"})
	if err == nil || !strings.Contains(err.Error(), "could not check") || *deletes != 0 {
		t.Fatalf("expected a safe refusal, got err=%v deletes=%d", err, *deletes)
	}
}

func TestFlowsFindExcludesTheFlowItself(t *testing.T) {
	ix := &usageIndex{flows: []indexedFlow{
		{id: "self", name: "Self", document: map[string]any{
			"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
			"conditions": []any{}, "actions": []any{map[string]any{"id": "homey:manager:flow:start", "args": map[string]any{"flow": map[string]any{"id": "self"}}}},
		}},
		{id: "caller", name: "Caller", document: map[string]any{
			"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
			"conditions": []any{}, "actions": []any{map[string]any{"id": "homey:manager:flow:start", "args": map[string]any{"flow": map[string]any{"id": "self"}}}},
		}},
	}}
	u := ix.find("self", "Self", "self")
	if len(u.Flows) != 1 || u.Flows[0].ID != "caller" {
		t.Fatalf("flows = %+v, want only the caller", u.Flows)
	}
}

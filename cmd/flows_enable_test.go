package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// enableServer serves one simple flow "one" and records writes. PUTs are
// applied so the read-back sees them, as Homey does.
type enableServer struct {
	flow   map[string]any
	writes []map[string]any
}

func (s *enableServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/manager/flow/flow/":
			_ = json.NewEncoder(w).Encode(map[string]any{"one": s.flow})
		case "GET /api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		case "GET /api/manager/flow/flow/one":
			_ = json.NewEncoder(w).Encode(s.flow)
		case "GET /api/manager/devices/device/":
			_, _ = w.Write([]byte(`{"lamp":{"id":"lamp","name":"Stue taklampe"}}`))
		case "PUT /api/manager/flow/flow/one":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.writes = append(s.writes, body)
			for k, v := range body {
				s.flow[k] = v
			}
			_ = json.NewEncoder(w).Encode(s.flow)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func newEnableServer(t *testing.T, enabled bool) *enableServer {
	t.Helper()
	s := &enableServer{flow: map[string]any{
		"id": "one", "name": "Night lights", "enabled": enabled,
		"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
		"conditions": []any{},
		"actions":    []any{map[string]any{"id": "homey:device:lamp:on", "args": map[string]any{}}},
	}}
	testFlowServer(t, s.handler(t))
	return s
}

func resetEnableFlags(t *testing.T) {
	t.Cleanup(func() {
		_ = flowsEnableCmd.Flags().Set("yes", "false")
		_ = flowsEnableCmd.Flags().Set("dry-run", "false")
		_ = flowsDisableCmd.Flags().Set("dry-run", "false")
	})
}

func TestEnableWithoutYesChangesNothing(t *testing.T) {
	s := newEnableServer(t, false)
	resetEnableFlags(t)
	backupFlow = func(string, json.RawMessage) (string, error) {
		t.Error("backup taken without --yes")
		return "", nil
	}
	err := flowsEnableCmd.RunE(flowsEnableCmd, []string{"one"})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected a --yes error, got %v", err)
	}
	if len(s.writes) != 0 {
		t.Fatalf("enable without --yes wrote %d times", len(s.writes))
	}
}

func TestEnableWithYesBacksUpThenWritesAndVerifies(t *testing.T) {
	s := newEnableServer(t, false)
	resetEnableFlags(t)
	backedUp := false
	backupFlow = func(string, json.RawMessage) (string, error) {
		if len(s.writes) != 0 {
			t.Error("write before backup")
		}
		backedUp = true
		return "/test/backup.json", nil
	}
	_ = flowsEnableCmd.Flags().Set("yes", "true")
	// A read-back failure would surface here as an error. This guards the
	// simple-flow case, where verifying against the bare {"enabled":true}
	// patch compares trigger/actions to nil and always fails.
	if err := flowsEnableCmd.RunE(flowsEnableCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if !backedUp || len(s.writes) != 1 || s.writes[0]["enabled"] != true {
		t.Fatalf("backedUp=%v writes=%v", backedUp, s.writes)
	}
	if s.writes[0]["trigger"] == nil {
		t.Fatal("enable sent a partial document and dropped the trigger")
	}
}

func TestEnableDryRunChangesNothing(t *testing.T) {
	s := newEnableServer(t, false)
	resetEnableFlags(t)
	backupFlow = func(string, json.RawMessage) (string, error) { t.Error("backup in dry run"); return "", nil }
	_ = flowsEnableCmd.Flags().Set("dry-run", "true")
	_ = flowsEnableCmd.Flags().Set("yes", "true") // --dry-run wins over --yes
	if err := flowsEnableCmd.RunE(flowsEnableCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if len(s.writes) != 0 {
		t.Fatal("dry run wrote")
	}
}

func TestEnableAlreadyEnabledIsNoop(t *testing.T) {
	s := newEnableServer(t, true)
	resetEnableFlags(t)
	backupFlow = func(string, json.RawMessage) (string, error) { t.Error("backup for a no-op"); return "", nil }
	_ = flowsEnableCmd.Flags().Set("yes", "true")
	if err := flowsEnableCmd.RunE(flowsEnableCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if len(s.writes) != 0 {
		t.Fatal("no-op enable wrote")
	}
}

func TestEnableBackupFailurePreventsWrite(t *testing.T) {
	s := newEnableServer(t, false)
	resetEnableFlags(t)
	backupFlow = func(string, json.RawMessage) (string, error) { return "", errors.New("disk full") }
	_ = flowsEnableCmd.Flags().Set("yes", "true")
	err := flowsEnableCmd.RunE(flowsEnableCmd, []string{"one"})
	if err == nil || !strings.Contains(err.Error(), "backup failed") || len(s.writes) != 0 {
		t.Fatalf("unsafe result writes=%d err=%v", len(s.writes), err)
	}
}

func TestDisableNeedsNoConfirmationAndIgnoresValidation(t *testing.T) {
	s := newEnableServer(t, true)
	resetEnableFlags(t)
	// Make the flow invalid: disabling must still be possible.
	s.flow["trigger"] = map[string]any{}
	backupFlow = func(string, json.RawMessage) (string, error) { return "/test/backup.json", nil }
	if err := flowsDisableCmd.RunE(flowsDisableCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if len(s.writes) != 1 || s.writes[0]["enabled"] != false {
		t.Fatalf("writes=%v", s.writes)
	}
}

func TestDescribeFlowEffectsAdvanced(t *testing.T) {
	doc := map[string]any{"cards": map[string]any{
		"a": map[string]any{"type": "trigger", "id": "homey:device:lamp:turned_on"},
		"b": map[string]any{"type": "condition", "id": "homey:manager:logic:lt"},
		"c": map[string]any{"type": "action", "id": "homey:device:gone:off"},
		"d": map[string]any{"type": "delay"},
		"e": map[string]any{"type": "note", "value": "ignored"},
		"f": map[string]any{"type": "start"},
	}}
	e := describeFlowEffects(doc, true).withDeviceNames(map[string]string{"lamp": "Stue taklampe"})
	want := flowEffects{
		Triggers:   []string{"Stue taklampe: turned_on", "start (manual / triggered by another flow)"},
		Conditions: []string{"homey:manager:logic:lt"},
		Actions:    []string{"delay", "homey:device:gone:off (device not found)"},
	}
	got, _ := json.Marshal(e)
	exp, _ := json.Marshal(want)
	if string(got) != string(exp) {
		t.Fatalf("effects\n got %s\nwant %s", got, exp)
	}
}

func TestWithDeviceNamesKeepsRawIDsWhenLookupFailed(t *testing.T) {
	e := flowEffects{Actions: []string{"homey:device:lamp:on"}}
	if got := e.withDeviceNames(nil).Actions[0]; got != "homey:device:lamp:on" {
		t.Fatalf("got %q; a failed lookup must not mark devices as missing", got)
	}
}

package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fishfisher/homeyctl/internal/client"
	"github.com/fishfisher/homeyctl/internal/config"
)

func testFlowServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	oldClient, oldBackup := apiClient, backupFlow
	apiClient = client.New(&config.Config{Mode: "local", Local: config.LocalConfig{Address: server.URL, Token: "test"}})
	t.Cleanup(func() { apiClient = oldClient; backupFlow = oldBackup })
}

func TestUpdateBacksUpCompleteFlowBeforeMutation(t *testing.T) {
	current := map[string]any{"id": "one", "name": "Before", "enabled": true, "trigger": map[string]any{"id": "homey:manager:flow:programmatic_trigger"}, "conditions": []any{}, "actions": []any{}}
	backedUp := false
	writes := 0
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/manager/flow/flow/":
			_ = json.NewEncoder(w).Encode(map[string]any{"one": map[string]any{"id": "one", "name": "Before"}})
		case "GET /api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		case "GET /api/manager/flow/flow/one":
			_ = json.NewEncoder(w).Encode(current)
		case "PUT /api/manager/flow/flow/one":
			if !backedUp {
				t.Error("mutation before backup")
			}
			writes++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["trigger"] == nil {
				t.Error("partial rename lost trigger")
			}
			current = body
			_ = json.NewEncoder(w).Encode(current)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	backupFlow = func(name string, raw json.RawMessage) (string, error) {
		var saved map[string]any
		_ = json.Unmarshal(raw, &saved)
		if saved["trigger"] == nil || saved["name"] != "Before" {
			t.Error("backup did not contain full original flow")
		}
		backedUp = true
		return "/test/backup.json", nil
	}
	_ = flowsUpdateCmd.Flags().Set("data", `{"name":"After"}`)
	t.Cleanup(func() { _ = flowsUpdateCmd.Flags().Set("data", "") })
	if err := flowsUpdateCmd.RunE(flowsUpdateCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("expected one write, got %d", writes)
	}
}

func TestUpdateBackupFailurePreventsWrite(t *testing.T) {
	writes := 0
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
			w.WriteHeader(500)
			return
		}
		switch r.URL.Path {
		case "/api/manager/flow/flow/":
			_, _ = w.Write([]byte(`{"one":{"id":"one","name":"Before"}}`))
		case "/api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"name":"Before","enabled":true,"trigger":{"id":"homey:manager:flow:programmatic_trigger"},"conditions":[],"actions":[]}`))
		}
	})
	backupFlow = func(string, json.RawMessage) (string, error) { return "", errors.New("disk full") }
	_ = flowsUpdateCmd.Flags().Set("data", `{"enabled":false}`)
	t.Cleanup(func() { _ = flowsUpdateCmd.Flags().Set("data", "") })
	err := flowsUpdateCmd.RunE(flowsUpdateCmd, []string{"one"})
	if err == nil || !strings.Contains(err.Error(), "backup failed") || writes != 0 {
		t.Fatalf("unsafe result writes=%d err=%v", writes, err)
	}
}

func TestAICreateDryRunDoesNotContactHomey(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("dry-run contacted Homey"); w.WriteHeader(500) })
	file := filepath.Join(t.TempDir(), "flow.json")
	if err := os.WriteFile(file, []byte(`{"name":"Review","enabled":true,"trigger":{"id":"homey:manager:flow:programmatic_trigger"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = flowsCreateCmd.Flags().Set("ai", "true")
	_ = flowsCreateCmd.Flags().Set("dry-run", "true")
	t.Cleanup(func() {
		_ = flowsCreateCmd.Flags().Set("ai", "false")
		_ = flowsCreateCmd.Flags().Set("dry-run", "false")
	})
	if err := flowsCreateCmd.RunE(flowsCreateCmd, []string{file}); err != nil {
		t.Fatal(err)
	}
}

func TestAICreateEnforcesDisabledAndFolder(t *testing.T) {
	var created map[string]any
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/manager/flow/flowfolder/":
			_, _ = w.Write([]byte(`{"review":{"id":"review","name":"AI Flows","parent":null}}`))
		case "POST /api/manager/flow/flow/":
			_ = json.NewDecoder(r.Body).Decode(&created)
			if created["enabled"] != false || created["folder"] != "review" {
				t.Errorf("unsafe draft: %#v", created)
			}
			created["id"] = "new"
			_ = json.NewEncoder(w).Encode(created)
		case "GET /api/manager/flow/flow/new":
			_ = json.NewEncoder(w).Encode(created)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	file := filepath.Join(t.TempDir(), "flow.json")
	_ = os.WriteFile(file, []byte(`{"name":"Review","enabled":true,"folder":"production","trigger":{"id":"homey:manager:flow:programmatic_trigger"}}`), 0o600)
	_ = flowsCreateCmd.Flags().Set("ai", "true")
	t.Cleanup(func() { _ = flowsCreateCmd.Flags().Set("ai", "false") })
	if err := flowsCreateCmd.RunE(flowsCreateCmd, []string{file}); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateFlowNamesRequireID(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/manager/flow/flow/" {
			_, _ = w.Write([]byte(`{"one":{"id":"one","name":"Same"}}`))
		} else {
			_, _ = w.Write([]byte(`{"two":{"id":"two","name":"same"}}`))
		}
	})
	if _, err := findFlow("Same"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity, got %v", err)
	}
}

func writeTempFlow(t *testing.T, document map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.json")
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRestoreBacksUpCurrentStateAndSendsCompleteDocument(t *testing.T) {
	current := map[string]any{
		"id": "one", "name": "Drifted", "enabled": true,
		"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
		"conditions": []any{map[string]any{"id": "homey:manager:logic:gt"}},
		"actions":    []any{},
	}
	backedUp := false
	writes := 0
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/manager/flow/flow/":
			_ = json.NewEncoder(w).Encode(map[string]any{"one": map[string]any{"id": "one", "name": "Drifted"}})
		case "GET /api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		case "GET /api/manager/flow/flow/one":
			_ = json.NewEncoder(w).Encode(current)
		case "PUT /api/manager/flow/flow/one":
			if !backedUp {
				t.Error("mutation before backup")
			}
			writes++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["id"] != nil {
				t.Error("restore forwarded server metadata")
			}
			conditions, _ := body["conditions"].([]any)
			if len(conditions) != 0 {
				t.Errorf("restore kept %d condition(s) added since the backup", len(conditions))
			}
			body["id"] = "one"
			current = body
			_ = json.NewEncoder(w).Encode(current)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	backupFlow = func(name string, raw json.RawMessage) (string, error) {
		var saved map[string]any
		_ = json.Unmarshal(raw, &saved)
		if saved["name"] != "Drifted" {
			t.Error("restore did not back up the current state first")
		}
		backedUp = true
		return "/test/backup.json", nil
	}

	path := writeTempFlow(t, map[string]any{
		"id": "one", "name": "Original", "enabled": true,
		"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
		"conditions": []any{},
		"actions":    []any{},
	})
	if err := flowsRestoreCmd.RunE(flowsRestoreCmd, []string{path}); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("expected one write, got %d", writes)
	}
	if current["name"] != "Original" {
		t.Fatalf("expected restored name, got %v", current["name"])
	}
}

func TestRestoreDryRunMakesNoWrites(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("dry run wrote to %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/manager/flow/flow/":
			_, _ = w.Write([]byte(`{"one":{"id":"one","name":"Drifted"}}`))
		case "/api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"id":"one","name":"Drifted","enabled":true,"trigger":{"id":"homey:manager:flow:programmatic_trigger"},"conditions":[],"actions":[]}`))
		}
	})
	backupFlow = func(string, json.RawMessage) (string, error) {
		t.Fatal("dry run created a backup")
		return "", nil
	}
	path := writeTempFlow(t, map[string]any{
		"id": "one", "name": "Original", "enabled": true,
		"trigger": map[string]any{"id": "homey:manager:flow:programmatic_trigger"}, "conditions": []any{}, "actions": []any{},
	})
	_ = flowsRestoreCmd.Flags().Set("dry-run", "true")
	t.Cleanup(func() { _ = flowsRestoreCmd.Flags().Set("dry-run", "false") })
	if err := flowsRestoreCmd.RunE(flowsRestoreCmd, []string{path}); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRejectsMismatchedFlowKind(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manager/flow/flow/":
			_, _ = w.Write([]byte(`{"one":{"id":"one","name":"Simple"}}`))
		case "/api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"id":"one","name":"Simple","enabled":true,"trigger":{"id":"homey:manager:flow:programmatic_trigger"},"conditions":[],"actions":[]}`))
		}
	})
	backupFlow = func(string, json.RawMessage) (string, error) {
		t.Fatal("wrote a backup despite a mismatched backup kind")
		return "", nil
	}
	path := writeTempFlow(t, map[string]any{"id": "one", "name": "Advanced", "cards": map[string]any{}})
	err := flowsRestoreCmd.RunE(flowsRestoreCmd, []string{path})
	if err == nil || !strings.Contains(err.Error(), "restore into a matching flow") {
		t.Fatalf("expected a flow-kind mismatch error, got %v", err)
	}
}

func TestRestoreRejectsNonFlowBackup(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manager/flow/flow/":
			_, _ = w.Write([]byte(`{"one":{"id":"one","name":"Simple"}}`))
		case "/api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"id":"one","name":"Simple","enabled":true,"trigger":{"id":"homey:manager:flow:programmatic_trigger"},"conditions":[],"actions":[]}`))
		}
	})
	backupFlow = func(string, json.RawMessage) (string, error) {
		t.Fatal("wrote a backup for a variable backup file")
		return "", nil
	}
	// A variables backup has the same shape on disk but is not a flow.
	path := writeTempFlow(t, map[string]any{"id": "one", "name": "Thermostat", "type": "number", "value": 21.5})
	err := flowsRestoreCmd.RunE(flowsRestoreCmd, []string{path})
	if err == nil || !strings.Contains(err.Error(), "is not a flow backup") {
		t.Fatalf("expected a not-a-flow-backup error, got %v", err)
	}
}

func TestRestoreWithoutIDRequiresExplicitTarget(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("contacted Homey before resolving the target: %s", r.URL.Path)
	})
	path := writeTempFlow(t, map[string]any{"name": "Nameless", "trigger": map[string]any{"id": "x"}})
	err := flowsRestoreCmd.RunE(flowsRestoreCmd, []string{path})
	if err == nil || !strings.Contains(err.Error(), "--to") {
		t.Fatalf("expected a missing-target error, got %v", err)
	}
}

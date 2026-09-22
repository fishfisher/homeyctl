package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchPlanDoesNotWrite(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("plan performed a write")
		}
		_, _ = w.Write([]byte(`{}`))
	})
	file := filepath.Join(t.TempDir(), "manifest.json")
	_ = os.WriteFile(file, []byte(`[{"name":"AI.Test","type":"boolean","value":false}]`), 0o600)
	if err := varsBatchCmd.RunE(varsBatchCmd, []string{file}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchFailureRetainsReceiptAndStops(t *testing.T) {
	writes := 0
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		writes++
		if writes == 1 {
			_, _ = w.Write([]byte(`{"id":"created-one","name":"AI.A","type":"boolean","value":false}`))
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	})
	dir := t.TempDir()
	file := filepath.Join(dir, "manifest.json")
	receipt := filepath.Join(dir, "receipt.json")
	_ = os.WriteFile(file, []byte(`[{"name":"AI.A","type":"boolean","value":false},{"name":"AI.B","type":"number","value":0},{"name":"AI.C","type":"string","value":"unused"}]`), 0o600)
	backupFlow = func(name string, raw json.RawMessage) (string, error) {
		return receipt, os.WriteFile(receipt, raw, 0o600)
	}
	_ = varsBatchCmd.Flags().Set("apply", "true")
	t.Cleanup(func() { _ = varsBatchCmd.Flags().Set("apply", "false") })
	err := varsBatchCmd.RunE(varsBatchCmd, []string{file})
	if err == nil || !strings.Contains(err.Error(), "batch stopped") || writes != 2 {
		t.Fatalf("unexpected result writes=%d err=%v", writes, err)
	}
	data, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["pending"] != "AI.B" || len(saved["created"].([]any)) != 1 {
		t.Fatalf("incomplete recovery information: %s", data)
	}
}

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestFolderRefToleratesNonStrings(t *testing.T) {
	for raw, want := range map[string]string{`"abc"`: "abc", `null`: "", `false`: "", `{"id":"x"}`: ""} {
		var f struct {
			Folder folderRef `json:"folder"`
		}
		if err := json.Unmarshal([]byte(`{"folder":`+raw+`}`), &f); err != nil || string(f.Folder) != want {
			t.Errorf("folder %s → %q, %v; want %q", raw, f.Folder, err, want)
		}
	}
}

func TestFlowListFilterKeep(t *testing.T) {
	on, off := true, false
	cases := []struct {
		name   string
		filter flowListFilter
		flow   string
		folder folderRef
		enable bool
		want   bool
	}{
		{"no filter keeps all", flowListFilter{}, "Any", "", false, true},
		{"match is case-insensitive", flowListFilter{match: "night"}, "Night lights", "", true, true},
		{"match excludes", flowListFilter{match: "morning"}, "Night lights", "", true, false},
		{"folder matches", flowListFilter{folderID: "ai"}, "Draft", "ai", false, true},
		{"folder excludes other", flowListFilter{folderID: "ai"}, "Draft", "other", false, false},
		{"folder excludes root", flowListFilter{folderID: "ai"}, "Draft", "", false, false},
		{"enabled only", flowListFilter{enabled: &on}, "X", "", false, false},
		{"disabled only", flowListFilter{enabled: &off}, "X", "", false, true},
		{"combined", flowListFilter{folderID: "ai", enabled: &off}, "X", "ai", false, true},
	}
	for _, tc := range cases {
		if got := tc.filter.keep(tc.flow, tc.folder, tc.enable); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestListDraftsInReviewFolder is the review workflow's question: which flows
// are waiting in "AI Flows"? Resolves the folder by name, filters both flow
// kinds, and must not return enabled flows or flows in other folders.
func TestListDraftsInReviewFolder(t *testing.T) {
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manager/flow/flowfolder/":
			_, _ = w.Write([]byte(`{"ai":{"id":"ai","name":"AI Flows"},"home":{"id":"home","name":"Home"}}`))
		case "/api/manager/flow/flow/":
			_, _ = w.Write([]byte(`{
				"d1":{"id":"d1","name":"Draft simple","enabled":false,"folder":"ai"},
				"e1":{"id":"e1","name":"Reviewed","enabled":true,"folder":"ai"},
				"h1":{"id":"h1","name":"Elsewhere","enabled":false,"folder":"home"},
				"r1":{"id":"r1","name":"Root","enabled":false,"folder":null}}`))
		case "/api/manager/flow/advancedflow/":
			_, _ = w.Write([]byte(`{"d2":{"id":"d2","name":"Draft advanced","enabled":false,"folder":"ai"}}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	oldJSON := jsonFlag
	jsonFlag = true
	t.Cleanup(func() {
		jsonFlag = oldJSON
		_ = flowsListCmd.Flags().Set("folder", "")
		_ = flowsListCmd.Flags().Set("disabled", "false")
	})
	_ = flowsListCmd.Flags().Set("folder", "ai flows") // by name, case-insensitive
	_ = flowsListCmd.Flags().Set("disabled", "true")

	out := captureStdout(t, func() {
		if err := flowsListCmd.RunE(flowsListCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	var got []FlowListItem
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", out, err)
	}
	if len(got) != 2 || got[0].ID != "d2" || got[1].ID != "d1" {
		t.Fatalf("drafts = %+v, want d2 (Draft advanced) and d1 (Draft simple)", got)
	}
	if got[0].Folder != "ai" {
		t.Fatalf("folder not reported in JSON: %+v", got[0])
	}
}

func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

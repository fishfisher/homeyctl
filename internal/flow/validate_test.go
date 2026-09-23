package flow

import "testing"

func TestValidateAdvancedFlow(t *testing.T) {
	triggerID := "00000000-0000-4000-8000-000000000001"
	allID := "00000000-0000-4000-8000-000000000002"
	actionID := "00000000-0000-4000-8000-000000000003"
	document := map[string]any{
		"name":    "Safe AI flow",
		"enabled": false,
		"cards": map[string]any{
			triggerID: map[string]any{
				"type":          "trigger",
				"id":            "homey:manager:flow:programmatic_trigger",
				"x":             float64(100),
				"y":             float64(100),
				"outputSuccess": []any{allID},
			},
			allID: map[string]any{
				"type":          "all",
				"x":             float64(450),
				"y":             float64(100),
				"input":         []any{triggerID + "::outputSuccess"},
				"outputSuccess": []any{actionID},
			},
			actionID: map[string]any{
				"type": "action",
				"id":   "homey:manager:notifications:create_notification",
				"x":    float64(800),
				"y":    float64(100),
			},
		},
	}

	report := Validate(document, true)
	if !report.Valid() {
		t.Fatalf("expected valid report, got errors: %#v", report.Errors)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
}

func TestValidateAdvancedFlowFindsBrokenGraph(t *testing.T) {
	document := map[string]any{
		"name": "Broken",
		"cards": map[string]any{
			"not-a-uuid": map[string]any{
				"type":          "trigger",
				"id":            "guessed-card",
				"x":             float64(0),
				"y":             float64(0),
				"outputSuccess": []any{"missing-card"},
			},
		},
	}

	report := Validate(document, true)
	if report.Valid() {
		t.Fatal("expected invalid report")
	}
	wantCodes := map[string]bool{
		"invalid_card_id":       false,
		"missing_output_target": false,
	}
	for _, issue := range report.Errors {
		if _, ok := wantCodes[issue.Code]; ok {
			wantCodes[issue.Code] = true
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Errorf("missing expected issue %q in %#v", code, report.Errors)
		}
	}
}

func TestValidateAdvancedAnyDoesNotRequireInput(t *testing.T) {
	triggerID := "00000000-0000-4000-8000-000000000001"
	anyID := "00000000-0000-4000-8000-000000000002"
	document := map[string]any{
		"name": "Any join",
		"cards": map[string]any{
			triggerID: map[string]any{
				"type":          "trigger",
				"id":            "homey:manager:flow:programmatic_trigger",
				"x":             float64(0),
				"y":             float64(0),
				"outputSuccess": []any{anyID},
			},
			anyID: map[string]any{
				"type": "any",
				"x":    float64(400),
				"y":    float64(0),
			},
		},
	}

	if report := Validate(document, true); !report.Valid() {
		t.Fatalf("any card without input must be valid: %#v", report.Errors)
	}
}

func TestMergeUpdate(t *testing.T) {
	current := map[string]any{
		"id":      "read-only",
		"name":    "Before",
		"enabled": true,
		"cards": map[string]any{
			"keep":   map[string]any{"type": "start"},
			"remove": map[string]any{"type": "note"},
		},
	}
	patch := map[string]any{
		"name": "After",
		"cards": map[string]any{
			"remove": nil,
			"add":    map[string]any{"type": "action"},
		},
	}

	merged := MergeUpdate(current, patch, true)
	if merged["name"] != "After" || merged["enabled"] != true {
		t.Fatalf("unexpected top-level merge: %#v", merged)
	}
	if _, exists := merged["id"]; exists {
		t.Fatal("read-only id must not be copied")
	}
	cards := merged["cards"].(map[string]any)
	if _, exists := cards["keep"]; !exists {
		t.Fatal("existing card was lost")
	}
	if _, exists := cards["remove"]; exists {
		t.Fatal("null patch did not remove card")
	}
	if _, exists := cards["add"]; !exists {
		t.Fatal("new card was not added")
	}
}

func TestEmbeddedTagSeparator(t *testing.T) {
	const id = "465af688-79ae-4161-b166-c2560c48ae4b"
	flow := func(text string) map[string]any {
		return map[string]any{"name": "Tags", "cards": map[string]any{
			"11111111-1111-4111-8111-111111111111": map[string]any{
				"type": "trigger", "id": "homey:manager:flow:programmatic_trigger", "x": 0, "y": 0,
				"outputSuccess": []any{"22222222-2222-4222-8222-222222222222"},
			},
			"22222222-2222-4222-8222-222222222222": map[string]any{
				"type": "action", "id": "homey:manager:notifications:create_notification", "x": 500, "y": 0,
				"args": map[string]any{"text": text},
			},
		}}
	}
	warned := func(r Report) bool {
		for _, w := range r.Warnings {
			if w.Code == "embedded_tag_separator" {
				return true
			}
		}
		return false
	}
	for _, ok := range []string{
		"Delta [[homey:manager:logic|" + id + "]] °C",
		"Track [[homey:device:" + id + "|speaker_track]]",
		"Azimuth [[homey:app:com.cyclone-software.sunevents|azimuth]]",
		"Local [[trigger::11111111-1111-4111-8111-111111111111::measure_temperature]]",
		"No tags at all",
	} {
		if r := Validate(flow(ok), true); warned(r) {
			t.Errorf("unexpected warning for %q: %+v", ok, r.Warnings)
		}
	}
	for _, bad := range []string{
		"Delta [[homey:manager:logic:" + id + "]] °C",
		"Track [[homey:device:" + id + ":speaker_track]]",
	} {
		if r := Validate(flow(bad), true); !warned(r) {
			t.Errorf("no warning for colon-form tag %q", bad)
		}
	}
	// Simple flows are checked too.
	simple := map[string]any{
		"name": "S", "trigger": map[string]any{"id": "homey:manager:flow:programmatic_trigger"}, "conditions": []any{},
		"actions": []any{map[string]any{"id": "homey:manager:notifications:create_notification", "args": map[string]any{"text": "[[homey:manager:logic:" + id + "]]"}}},
	}
	if r := Validate(simple, false); !warned(r) {
		t.Error("no warning for colon-form tag in a simple flow")
	}
}

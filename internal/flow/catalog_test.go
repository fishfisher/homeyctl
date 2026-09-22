package flow

import "testing"

func TestCatalogDoesNotConfuseAppAndLogicVariables(t *testing.T) {
	catalog := Catalog{Cards: map[string]map[string]bool{"trigger": {"homey:manager:flow:programmatic_trigger": true}, "action": {"homey:app:custom:set": true, "homey:manager:logic:variable_set_number": true}}, Variables: map[string]Variable{}}
	document := map[string]any{"name": "Test", "trigger": map[string]any{"id": "homey:manager:flow:programmatic_trigger"}, "conditions": []any{}, "actions": []any{map[string]any{"id": "homey:app:custom:set", "args": map[string]any{"variable": map[string]any{"id": "app-private-variable"}}}}}
	report := Validate(document, false)
	catalog.Validate(document, &report)
	if !report.Valid() {
		t.Fatalf("app variable treated as Logic: %#v", report.Errors)
	}
	document["actions"].([]any)[0].(map[string]any)["id"] = "homey:manager:logic:variable_set_number"
	report = Validate(document, false)
	catalog.Validate(document, &report)
	if report.Valid() {
		t.Fatal("missing Logic variable was accepted")
	}
}

func TestExportedNullDroptokenAndLocalTokens(t *testing.T) {
	document := map[string]any{"name": "Export", "trigger": map[string]any{"id": "homey:manager:logic:variable_changed"}, "conditions": []any{map[string]any{"id": "homey:manager:logic:equal", "droptoken": "value"}}, "actions": []any{map[string]any{"id": "homey:manager:notifications:create_notification", "droptoken": nil}}}
	if report := Validate(document, false); !report.Valid() {
		t.Fatalf("valid exported flow rejected: %#v", report.Errors)
	}
}

func TestDeletionProducesCompleteGraphWithoutNullCards(t *testing.T) {
	current := map[string]any{"cards": map[string]any{"keep": map[string]any{"type": "start"}, "remove": map[string]any{"type": "note"}}}
	patch := map[string]any{"cards": map[string]any{"remove": nil}}
	merged := MergeUpdate(current, patch, true)
	if _, exists := merged["cards"].(map[string]any)["remove"]; exists {
		t.Fatal("deleted card remains in payload")
	}
	if current["cards"].(map[string]any)["remove"] == nil {
		t.Fatal("original backup document was mutated")
	}
}

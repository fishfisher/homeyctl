package flow

import (
	"reflect"
	"testing"
)

const (
	lamp  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	vari  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	other = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	keyA  = "11111111-1111-4111-8111-111111111111"
	keyB  = "22222222-2222-4222-8222-222222222222"
)

func TestFindReferencesAdvancedCoversEveryForm(t *testing.T) {
	doc := map[string]any{"id": lamp, "name": "Own id is metadata, not a reference", "cards": map[string]any{
		keyA: map[string]any{
			"type": "trigger", "id": "homey:device:" + lamp + ":measure_temperature_changed", "x": 0, "y": 0,
			"outputSuccess": []any{keyB},
		},
		keyB: map[string]any{
			"type": "action", "id": "homey:manager:logic:variable_set_number_math", "x": 420, "y": 0,
			"droptoken": "homey:device:" + lamp + "|measure_temperature",
			"args": map[string]any{
				"variable": map[string]any{"id": vari, "name": "Delta"},
				"value":    "[[homey:device:" + lamp + "|measure_temperature]] - [[homey:manager:logic|" + vari + "]]",
			},
		},
	}}

	got := FindReferences(doc, true, lamp)
	want := []Reference{
		{Card: keyA, CardType: "trigger", CardID: "homey:device:" + lamp + ":measure_temperature_changed", Field: "id"},
		{Card: keyB, CardType: "action", CardID: "homey:manager:logic:variable_set_number_math", Field: "args.value"},
		{Card: keyB, CardType: "action", CardID: "homey:manager:logic:variable_set_number_math", Field: "droptoken"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("device refs\n got %+v\nwant %+v", got, want)
	}

	vars := FindReferences(doc, true, vari)
	if len(vars) != 2 || vars[0].Field != "args.value" || vars[1].Field != "args.variable.id" {
		t.Fatalf("variable refs = %+v", vars)
	}
	if refs := FindReferences(doc, true, other); len(refs) != 0 {
		t.Fatalf("unrelated id matched: %+v", refs)
	}
}

func TestFindReferencesIgnoresWiring(t *testing.T) {
	// Searching for a card key must not report the edges that point at it:
	// those are canvas wiring, not a reference to an external object.
	doc := map[string]any{"cards": map[string]any{
		keyA: map[string]any{"type": "trigger", "id": "homey:manager:flow:programmatic_trigger", "outputSuccess": []any{keyB}},
		keyB: map[string]any{"type": "all", "input": []any{keyA + "::outputSuccess"}},
	}}
	if refs := FindReferences(doc, true, keyB); len(refs) != 0 {
		t.Fatalf("wiring reported as references: %+v", refs)
	}
}

func TestFindReferencesSimpleFlow(t *testing.T) {
	doc := map[string]any{
		"trigger":    map[string]any{"id": "homey:manager:flow:programmatic_trigger"},
		"conditions": []any{map[string]any{"id": "homey:manager:logic:lt", "droptoken": "homey:manager:logic|" + vari}},
		"actions":    []any{map[string]any{"id": "homey:device:" + lamp + ":on"}, map[string]any{"id": "homey:device:" + lamp + ":off"}},
	}
	got := FindReferences(doc, false, lamp)
	if len(got) != 2 || got[0].Card != "actions[0]" || got[1].Card != "actions[1]" {
		t.Fatalf("simple device refs = %+v", got)
	}
	// Case-insensitive, as IDs are sometimes upper-cased in hand-written JSON.
	if got := FindReferences(doc, false, "BBBBBBBB-BBBB-4BBB-8BBB-BBBBBBBBBBBB"); len(got) != 1 || got[0].Card != "conditions[0]" {
		t.Fatalf("simple variable refs = %+v", got)
	}
}

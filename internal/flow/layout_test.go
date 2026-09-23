package flow

import (
	"encoding/json"
	"testing"
)

func card(typ string, x, y float64, extra map[string]any) map[string]any {
	c := map[string]any{"type": typ, "x": x, "y": y}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func out(keys ...string) []any {
	list := make([]any, len(keys))
	for i, k := range keys {
		list[i] = k
	}
	return list
}

func pos(t *testing.T, doc map[string]any, key string) (float64, float64) {
	t.Helper()
	c := doc["cards"].(map[string]any)[key].(map[string]any)
	x, _ := numeric(c["x"])
	y, _ := numeric(c["y"])
	return x, y
}

// crowded is a small flow deliberately placed on top of itself: a trigger
// feeding a condition whose true and false branches rejoin through ANY, plus
// a note, all within a few units of each other.
func crowded() map[string]any {
	return map[string]any{"cards": map[string]any{
		"trig": card("trigger", 0, 0, map[string]any{"id": "homey:manager:flow:programmatic_trigger", "outputSuccess": out("cond")}),
		"cond": card("condition", 10, 10, map[string]any{
			"id": "homey:manager:logic:lt", "args": map[string]any{"comparator": 1},
			"outputTrue": out("yes"), "outputFalse": out("no"),
		}),
		"yes":  card("action", 20, 20, map[string]any{"id": "homey:manager:notifications:create_notification", "args": map[string]any{"text": "yes"}, "outputSuccess": out("join")}),
		"no":   card("action", 20, 30, map[string]any{"id": "homey:manager:notifications:create_notification", "args": map[string]any{"text": "no"}, "outputSuccess": out("join")}),
		"join": card("any", 30, 30, map[string]any{"outputSuccess": out("end")}),
		"end":  card("delay", 40, 40, map[string]any{"args": map[string]any{"delay": map[string]any{"number": "5", "multiplier": 1}}}),
		"note": card("note", 5, -100, map[string]any{"value": "Explains the condition", "color": "yellow"}),
	}}
}

func TestLayoutRemovesOverlaps(t *testing.T) {
	doc := crowded()
	before := Report{}
	validateOverlaps(doc["cards"].(map[string]any), &before)
	if len(before.Warnings) == 0 {
		t.Fatal("fixture should start out overlapping")
	}
	result := Layout(doc)
	if result.Overlaps != 0 {
		t.Fatalf("%d overlaps remain after layout", result.Overlaps)
	}
	after := Report{}
	validateOverlaps(doc["cards"].(map[string]any), &after)
	if len(after.Warnings) != 0 {
		t.Fatalf("overlap warnings after layout: %+v", after.Warnings)
	}
}

func TestLayoutColumnsFollowExecutionOrder(t *testing.T) {
	doc := crowded()
	Layout(doc)
	order := []string{"trig", "cond", "yes", "join", "end"}
	for i := 1; i < len(order); i++ {
		x0, _ := pos(t, doc, order[i-1])
		x1, _ := pos(t, doc, order[i])
		if x1-x0 != LayoutColumnPitch {
			t.Errorf("%s→%s: dx=%v, want %v", order[i-1], order[i], x1-x0, LayoutColumnPitch)
		}
	}
}

func TestLayoutKeepsLinearChainLevel(t *testing.T) {
	doc := map[string]any{"cards": map[string]any{
		"a": card("trigger", 0, 500, map[string]any{"id": "x:y:t", "outputSuccess": out("b")}),
		"b": card("action", 0, 0, map[string]any{"id": "x:y:a", "outputSuccess": out("c")}),
		"c": card("action", 0, 900, map[string]any{"id": "x:y:a"}),
	}}
	Layout(doc)
	// Same-size cards on one chain share a center, hence a top edge.
	_, ya := pos(t, doc, "a")
	_, yb := pos(t, doc, "b")
	_, yc := pos(t, doc, "c")
	if ya != yb || yb != yc {
		t.Fatalf("chain not level: %v %v %v", ya, yb, yc)
	}
}

func TestLayoutStacksTrueAboveFalse(t *testing.T) {
	doc := crowded()
	// Author placed false above true; port order must win.
	doc["cards"].(map[string]any)["no"].(map[string]any)["y"] = -500.0
	Layout(doc)
	_, yes := pos(t, doc, "yes")
	_, no := pos(t, doc, "no")
	if !(yes < no) {
		t.Fatalf("true branch (y=%v) should be above false branch (y=%v)", yes, no)
	}
}

func TestLayoutKeepsNoteAboveItsCard(t *testing.T) {
	// Spread out the way a person builds it, with the note just above the
	// condition; the layout then moves everything.
	doc := crowded()
	cards := doc["cards"].(map[string]any)
	for key, xy := range map[string][2]float64{
		"trig": {0, 300}, "cond": {500, 300}, "yes": {1000, 200}, "no": {1000, 450},
		"join": {1500, 300}, "end": {1800, 300}, "note": {500, 180},
	} {
		cards[key].(map[string]any)["x"] = xy[0]
		cards[key].(map[string]any)["y"] = xy[1]
	}
	Layout(doc)
	nx, ny := pos(t, doc, "note")
	cx, cy := pos(t, doc, "cond")
	_, nh := EstimateSize(cards["note"].(map[string]any))
	if nx != cx {
		t.Fatalf("note x=%v, want its card's column x=%v", nx, cx)
	}
	if ny+nh+LayoutGap > cy+0.5 {
		t.Fatalf("note (y=%v h=%v) not above its card (y=%v)", ny, nh, cy)
	}
}

func TestLayoutPutsWideNoteAboveGraph(t *testing.T) {
	doc := crowded()
	cards := doc["cards"].(map[string]any)
	cards["note"].(map[string]any)["width"] = 1200.0
	Layout(doc)
	_, ny := pos(t, doc, "note")
	_, nh := EstimateSize(cards["note"].(map[string]any))
	for key, raw := range cards {
		if key == "note" {
			continue
		}
		if y, _ := numeric(raw.(map[string]any)["y"]); ny+nh > y {
			t.Fatalf("wide note bottom %v below card %s at %v", ny+nh, key, y)
		}
	}
}

func TestLayoutChangesOnlyPositions(t *testing.T) {
	doc := crowded()
	before, _ := json.Marshal(stripXY(doc))
	Layout(doc)
	after, _ := json.Marshal(stripXY(doc))
	if string(before) != string(after) {
		t.Fatalf("layout changed more than x/y:\n%s\n%s", before, after)
	}
}

func TestLayoutSurvivesCycle(t *testing.T) {
	doc := map[string]any{"cards": map[string]any{
		"t": card("trigger", 0, 0, map[string]any{"id": "x:y:t", "outputSuccess": out("a")}),
		"a": card("action", 0, 0, map[string]any{"id": "x:y:a", "outputSuccess": out("d")}),
		"d": card("delay", 0, 0, map[string]any{"args": map[string]any{"delay": map[string]any{"number": "1", "multiplier": 60}}, "outputSuccess": out("a")}),
	}}
	if result := Layout(doc); result.Overlaps != 0 || result.Columns != 3 {
		t.Fatalf("cycle layout: %+v", result)
	}
}

func TestOverlapWarningCalibration(t *testing.T) {
	cards := map[string]any{
		// Placed 320 apart; cards are ~340 wide, so these overlap (the spacing
		// of the first AI-built heating draft).
		"a": card("action", 0, 0, map[string]any{"id": "x:y:a", "outputSuccess": out("b")}),
		"b": card("action", 320, 0, map[string]any{"id": "x:y:a"}),
		// A delay pill 150 to the right of a card's start is how people lay
		// them out by hand; it must not warn.
		"c": card("action", 0, 400, map[string]any{"id": "x:y:a", "outputSuccess": out("d")}),
		"d": card("delay", 360, 400, map[string]any{"args": map[string]any{"delay": map[string]any{"number": "1", "multiplier": 1}}}),
		"e": card("delay", 150+360, 400, map[string]any{"args": map[string]any{"delay": map[string]any{"number": "1", "multiplier": 1}}}),
	}
	report := Report{}
	validateOverlaps(cards, &report)
	if len(report.Warnings) != 1 || report.Warnings[0].Path != "cards.a" {
		t.Fatalf("warnings = %+v, want exactly a/b", report.Warnings)
	}
}

func stripXY(doc map[string]any) map[string]any {
	copyCards := map[string]any{}
	for k, raw := range doc["cards"].(map[string]any) {
		c := map[string]any{}
		for field, v := range raw.(map[string]any) {
			if field != "x" && field != "y" {
				c[field] = v
			}
		}
		copyCards[k] = c
	}
	return map[string]any{"cards": copyCards}
}

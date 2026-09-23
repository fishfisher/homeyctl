package flow

import (
	"fmt"
	"sort"
	"strings"
)

// Reference is one place in a flow that mentions a target ID.
type Reference struct {
	// Card is the canvas key of an Advanced Flow card, or the position of a
	// simple-flow card: "trigger", "conditions[0]", "actions[2]".
	Card     string `json:"card"`
	CardType string `json:"cardType,omitempty"`
	// CardID is the installed card, e.g. "homey:device:<id>:on".
	CardID string `json:"cardId,omitempty"`
	// Field is where in the card the ID appears: "id", "droptoken",
	// "args.variable.id", "args.text", ...
	Field string `json:"field"`
}

// FindReferences lists every card field in a flow document that contains
// target. IDs are UUIDs, so a case-insensitive substring match is exact in
// practice and covers every form Homey uses: card IDs
// (homey:device:<id>:on), droptokens (homey:manager:logic|<id>), embedded
// tags ([[homey:device:<id>|measure_temperature]]) and autocomplete objects
// ({"id":"<id>"}). Only cards are searched, never the flow's own metadata.
func FindReferences(document map[string]any, advanced bool, target string) []Reference {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return nil
	}
	var refs []Reference
	visit := func(key string, card map[string]any) {
		cardType, _ := card["type"].(string)
		cardID, _ := card["id"].(string)
		for _, field := range matchingFields(card, "", target) {
			refs = append(refs, Reference{Card: key, CardType: cardType, CardID: cardID, Field: field})
		}
	}

	if advanced {
		cards, _ := document["cards"].(map[string]any)
		keys := make([]string, 0, len(cards))
		for key := range cards {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if card, ok := cards[key].(map[string]any); ok {
				visit(key, card)
			}
		}
		return refs
	}

	if trigger, ok := document["trigger"].(map[string]any); ok {
		visit("trigger", trigger)
	}
	for _, field := range []string{"conditions", "actions"} {
		list, _ := document[field].([]any)
		for i, raw := range list {
			if card, ok := raw.(map[string]any); ok {
				visit(fmt.Sprintf("%s[%d]", field, i), card)
			}
		}
	}
	return refs
}

// Card fields that hold canvas wiring, never an external ID.
var wiringFields = map[string]bool{
	"outputSuccess": true, "outputTrue": true, "outputFalse": true, "outputError": true,
	"input": true, "x": true, "y": true,
}

func matchingFields(value any, path, target string) []string {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			if path == "" && wiringFields[key] {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var out []string
		for _, key := range keys {
			next := key
			if path != "" {
				next = path + "." + key
			}
			out = append(out, matchingFields(v[key], next, target)...)
		}
		return out
	case []any:
		var out []string
		for i, child := range v {
			out = append(out, matchingFields(child, fmt.Sprintf("%s[%d]", path, i), target)...)
		}
		return out
	case string:
		if strings.Contains(strings.ToLower(v), target) {
			return []string{path}
		}
	}
	return nil
}

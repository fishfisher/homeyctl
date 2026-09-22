package flow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var advancedCardTypes = map[string]bool{
	"trigger":   true,
	"condition": true,
	"action":    true,
	"start":     true,
	"delay":     true,
	"all":       true,
	"any":       true,
	"note":      true,
}

var outputFields = []string{"outputSuccess", "outputTrue", "outputFalse", "outputError"}

// Issue is one deterministic validation finding.
type Issue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

// Report contains all findings for a flow document.
type Report struct {
	Advanced bool    `json:"advanced"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

func (r *Report) addError(code, path, message string) {
	r.Errors = append(r.Errors, Issue{Severity: "error", Code: code, Path: path, Message: message})
}

func (r *Report) addWarning(code, path, message string) {
	r.Warnings = append(r.Warnings, Issue{Severity: "warning", Code: code, Path: path, Message: message})
}

// Valid reports whether validation found no errors.
func (r Report) Valid() bool {
	return len(r.Errors) == 0
}

// Validate checks a complete simple or advanced flow document.
func Validate(document map[string]any, advanced bool) Report {
	report := Report{Advanced: advanced, Errors: []Issue{}, Warnings: []Issue{}}
	validateTopLevel(document, &report)
	if advanced {
		validateAdvanced(document, &report)
	} else {
		validateSimple(document, &report)
	}
	return report
}

func validateTopLevel(document map[string]any, report *Report) {
	name, ok := document["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		report.addError("required", "name", "name is required and must be a non-empty string")
	}
	if enabled, exists := document["enabled"]; exists {
		if _, ok := enabled.(bool); !ok {
			report.addError("type", "enabled", "enabled must be a boolean")
		}
	}
	if folder, exists := document["folder"]; exists && folder != nil {
		if _, ok := folder.(string); !ok {
			report.addError("type", "folder", "folder must be a folder ID string or null")
		}
	}
}

func validateSimple(document map[string]any, report *Report) {
	trigger, ok := objectAt(document, "trigger")
	if !ok {
		report.addError("required", "trigger", "trigger is required and must be an object")
	} else {
		validateCardID(trigger, "trigger.id", report)
	}

	validateSimpleCards(document, "conditions", report)
	validateSimpleCards(document, "actions", report)
}

func validateSimpleCards(document map[string]any, field string, report *Report) {
	raw, exists := document[field]
	if !exists {
		report.addError("required", field, field+" is required and must be an array")
		return
	}
	cards, ok := raw.([]any)
	if !ok {
		report.addError("type", field, field+" must be an array")
		return
	}
	for index, rawCard := range cards {
		card, ok := rawCard.(map[string]any)
		path := fmt.Sprintf("%s[%d]", field, index)
		if !ok {
			report.addError("type", path, "card must be an object")
			continue
		}
		validateCardID(card, path+".id", report)
		if droptoken, exists := card["droptoken"]; exists {
			validateDroptoken(droptoken, path+".droptoken", report)
		}
	}
}

func validateAdvanced(document map[string]any, report *Report) {
	cards, ok := objectAt(document, "cards")
	if !ok {
		report.addError("required", "cards", "cards is required and must be an object keyed by UUID")
		return
	}
	if len(cards) == 0 {
		report.addWarning("empty_flow", "cards", "advanced flow contains no cards")
		return
	}

	edges := make(map[string][]string, len(cards))
	roots := make([]string, 0)

	cardIDs := make([]string, 0, len(cards))
	for cardID := range cards {
		cardIDs = append(cardIDs, cardID)
	}
	sort.Strings(cardIDs)

	for _, cardID := range cardIDs {
		path := "cards." + cardID
		if !uuidPattern.MatchString(cardID) {
			report.addWarning("non_uuid_key", path, "prefer UUID card keys for new flows; preserve existing custom keys when editing")
		}

		card, ok := cards[cardID].(map[string]any)
		if !ok {
			report.addError("type", path, "card must be an object")
			continue
		}

		cardType, _ := card["type"].(string)
		if !advancedCardTypes[cardType] {
			report.addError("invalid_card_type", path+".type", "type must be trigger, condition, action, start, delay, all, any, or note")
			continue
		}
		if cardType == "trigger" || cardType == "start" {
			roots = append(roots, cardID)
		}

		validateCoordinates(card, path, report)
		validateAdvancedCardFields(cardID, cardType, card, cards, report)

		for _, field := range outputFields {
			targets, exists := stringArray(card[field])
			if !exists {
				if _, present := card[field]; present {
					report.addError("type", path+"."+field, "output must be an array of card UUID strings")
				}
				continue
			}
			validPort := (cardType == "condition" && (field == "outputTrue" || field == "outputFalse" || field == "outputError")) || (cardType == "action" && (field == "outputSuccess" || field == "outputError")) || (cardType != "condition" && cardType != "action" && cardType != "note" && field == "outputSuccess")
			if !validPort && len(targets) > 0 {
				report.addError("invalid_output_port", path+"."+field, "output port is not supported by this card type")
			}
			for index, target := range targets {
				if _, found := cards[target]; !found {
					report.addError("missing_output_target", fmt.Sprintf("%s.%s[%d]", path, field, index), "output points to a card that does not exist: "+target)
					continue
				}
				if targetCard, ok := cards[target].(map[string]any); ok && (targetCard["type"] == "note" || targetCard["type"] == "trigger" || targetCard["type"] == "start") {
					report.addError("invalid_output_target", path+"."+field, "execution cannot target a note or entrypoint card")
				}
				edges[cardID] = append(edges[cardID], target)
			}
		}
	}

	validateReachability(cards, edges, roots, report)
}

func validateAdvancedCardFields(cardID, cardType string, card, cards map[string]any, report *Report) {
	path := "cards." + cardID
	switch cardType {
	case "trigger", "condition", "action":
		validateCardID(card, path+".id", report)
		if args, exists := card["args"]; exists {
			if _, ok := args.(map[string]any); !ok {
				report.addError("type", path+".args", "args must be an object")
			}
		}
		if droptoken, exists := card["droptoken"]; exists {
			validateDroptoken(droptoken, path+".droptoken", report)
		}
	case "delay":
		args, ok := objectAt(card, "args")
		if !ok {
			report.addError("required", path+".args", "delay card requires args.delay")
			return
		}
		delay, ok := objectAt(args, "delay")
		if !ok {
			report.addError("required", path+".args.delay", "delay card requires args.delay")
			return
		}
		number, ok := delay["number"].(string)
		if !ok || strings.TrimSpace(number) == "" {
			report.addError("type", path+".args.delay.number", "delay number must be a non-empty string")
		}
		multiplier, ok := numeric(delay["multiplier"])
		if !ok || (multiplier != 1 && multiplier != 60) {
			report.addError("invalid_delay_multiplier", path+".args.delay.multiplier", "delay multiplier must be 1 (seconds) or 60 (minutes)")
		}
	case "all":
		inputs, ok := stringArray(card["input"])
		if !ok || len(inputs) == 0 {
			report.addError("required", path+".input", "all card requires at least one input reference")
			return
		}
		for index, input := range inputs {
			validateInputReference(cardID, input, fmt.Sprintf("%s.input[%d]", path, index), cards, report)
		}
	case "note":
		if _, ok := card["value"].(string); !ok {
			report.addError("required", path+".value", "note value must be a string")
		}
		if color, ok := card["color"].(string); !ok || !map[string]bool{"yellow": true, "red": true, "green": true, "blue": true}[color] {
			report.addError("invalid_note_color", path+".color", "note color must be yellow, red, green, or blue")
		}
	}
}

func validateCoordinates(card map[string]any, path string, report *Report) {
	for _, field := range []string{"x", "y"} {
		if _, ok := numeric(card[field]); !ok {
			report.addError("type", path+"."+field, field+" must be a number")
		}
	}
}

func validateCardID(card map[string]any, path string, report *Report) {
	id, ok := card["id"].(string)
	if !ok || strings.TrimSpace(id) == "" {
		report.addError("required", path, "card id is required and must be a non-empty string")
		return
	}
	if !strings.HasPrefix(id, "homey:") {
		report.addError("invalid_card_id", path, "card id must start with homey:")
	}
}

func validateDroptoken(raw any, path string, report *Report) {
	if raw == nil {
		return
	} // Homey exports optional droptokens as null.
	token, ok := raw.(string)
	if !ok || strings.TrimSpace(token) == "" {
		report.addError("type", path, "droptoken must be a non-empty string")
		return
	}
	if !report.Advanced && !strings.HasPrefix(token, "homey:") {
		return
	} // Simple-flow trigger token.
	if !strings.Contains(token, "|") && !strings.HasPrefix(token, "trigger::") && !strings.HasPrefix(token, "action::") {
		report.addError("invalid_droptoken", path, "droptoken must use ownerUri|token or trigger::<card-uuid>::<token>")
	}
}

func validateInputReference(targetID, input, path string, cards map[string]any, report *Report) {
	parts := strings.Split(input, "::")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		report.addError("invalid_input_reference", path, "input must use <card-uuid>::<output-port>")
		return
	}
	sourceID, port := parts[0], parts[1]
	source, ok := cards[sourceID].(map[string]any)
	if !ok {
		report.addError("missing_input_source", path, "input source card does not exist: "+sourceID)
		return
	}
	if !contains(outputFields, port) {
		report.addError("invalid_input_port", path, "input port must be outputSuccess, outputTrue, outputFalse, or outputError")
		return
	}
	targets, ok := stringArray(source[port])
	if !ok || !contains(targets, targetID) {
		report.addError("unlinked_input", path, "source port does not point back to this all card")
	}
}

func validateReachability(cards map[string]any, edges map[string][]string, roots []string, report *Report) {
	if len(roots) == 0 {
		report.addError("missing_entrypoint", "cards", "advanced flow needs at least one trigger or start card")
		return
	}
	visited := make(map[string]bool, len(cards))
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		queue = append(queue, edges[current]...)
	}
	cardIDs := make([]string, 0, len(cards))
	for id := range cards {
		cardIDs = append(cardIDs, id)
	}
	sort.Strings(cardIDs)
	for _, cardID := range cardIDs {
		raw := cards[cardID]
		card, ok := raw.(map[string]any)
		if !ok || card["type"] == "note" || visited[cardID] {
			continue
		}
		report.addWarning("orphan_card", "cards."+cardID, "card is not reachable from a trigger or start card")
	}
}

func objectAt(value map[string]any, key string) (map[string]any, bool) {
	object, ok := value[key].(map[string]any)
	return object, ok
}

func stringArray(raw any) ([]string, bool) {
	if raw == nil {
		return nil, false
	}
	values, ok := raw.([]any)
	if !ok {
		if stringsValue, ok := raw.([]string); ok {
			return stringsValue, true
		}
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}

func numeric(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

func contains[T comparable](values []T, target T) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

package flow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Variable struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Catalog holds read-only discovery data from the target Homey.
type Catalog struct {
	Cards     map[string]map[string]bool
	Variables map[string]Variable
}

var logicTokenPattern = regexp.MustCompile(`homey:manager:logic[|:]([0-9a-fA-F-]{36})`)

// Validate checks references, never invokes cards or modifies variables.
func (c *Catalog) Validate(document map[string]any, report *Report) {
	checkCard := func(card map[string]any, kind, path string) {
		id, _ := card["id"].(string)
		if id != "" && !c.Cards[kind][id] {
			report.addError("unknown_card", path+".id", fmt.Sprintf("%s card is not available on this Homey: %s", kind, id))
		}
		if strings.HasPrefix(id, "homey:manager:logic:") {
			if args, ok := card["args"].(map[string]any); ok {
				if raw, exists := args["variable"]; exists {
					ref, ok := raw.(map[string]any)
					variableID, _ := ref["id"].(string)
					if !ok || variableID == "" {
						report.addError("invalid_variable_reference", path+".args.variable", "Logic variable must be an autocomplete object with an id")
					} else if variable, found := c.Variables[variableID]; !found {
						report.addError("unknown_variable", path+".args.variable", "Logic variable does not exist: "+variableID)
					} else {
						for _, kind := range []string{"boolean", "number", "string"} {
							if strings.Contains(id, "variable_set_"+kind) && variable.Type != kind {
								report.addError("variable_type_mismatch", path+".args.variable", "card requires a "+kind+" variable")
							}
						}
					}
				}
			}
		}
		c.walkReferences(card["args"], path+".args", report)
		c.walkReferences(card["droptoken"], path+".droptoken", report)
	}
	if report.Advanced {
		cards, _ := document["cards"].(map[string]any)
		keys := make([]string, 0, len(cards))
		for key := range cards {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			card, _ := cards[key].(map[string]any)
			kind, _ := card["type"].(string)
			if kind == "trigger" || kind == "condition" || kind == "action" {
				checkCard(card, kind, "cards."+key)
			}
		}
	} else {
		if trigger, ok := document["trigger"].(map[string]any); ok {
			checkCard(trigger, "trigger", "trigger")
		}
		for field, kind := range map[string]string{"conditions": "condition", "actions": "action"} {
			cards, _ := document[field].([]any)
			for index, raw := range cards {
				if card, ok := raw.(map[string]any); ok {
					checkCard(card, kind, fmt.Sprintf("%s[%d]", field, index))
				}
			}
		}
	}
}

func (c *Catalog) walkReferences(value any, path string, report *Report) {
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := strings.TrimPrefix(path+"."+key, ".")
			c.walkReferences(value[key], nextPath, report)
		}
	case []any:
		for i, child := range value {
			c.walkReferences(child, fmt.Sprintf("%s[%d]", path, i), report)
		}
	case string:
		for _, match := range logicTokenPattern.FindAllStringSubmatch(value, -1) {
			if _, found := c.Variables[match[1]]; !found {
				report.addError("unknown_variable", path, "Logic token refers to a missing variable: "+match[1])
			}
		}
	}
}

package flow

import "fmt"

// ValidatePatch rejects malformed or misspelled fields rather than silently ignoring them.
func ValidatePatch(patch map[string]any, advanced bool) error {
	if len(patch) == 0 {
		return fmt.Errorf("update must be a non-empty JSON object")
	}
	allowed := map[string]bool{"name": true, "folder": true, "enabled": true, "trigger": !advanced, "conditions": !advanced, "actions": !advanced, "cards": advanced}
	// Server metadata can appear in exported flow documents but is never written.
	metadata := map[string]bool{"id": true, "broken": true, "triggerable": true, "editable": true, "runnable": true}
	for key, value := range patch {
		if metadata[key] {
			continue
		}
		if !allowed[key] {
			return fmt.Errorf("unsupported flow update field %q", key)
		}
		if key == "cards" {
			if _, ok := value.(map[string]any); !ok {
				return fmt.Errorf("cards must be an object keyed by UUID")
			}
		}
	}
	return nil
}

// MergeUpdate overlays a partial update on the current server document and
// returns a complete API payload. Advanced card entries are merged by card UUID;
// setting a card UUID to null removes that card.
func MergeUpdate(current, patch map[string]any, advanced bool) map[string]any {
	allowed := []string{"name", "folder", "enabled", "trigger", "conditions", "actions"}
	if advanced {
		allowed = []string{"name", "folder", "enabled", "cards"}
	}

	merged := make(map[string]any, len(allowed))
	for _, key := range allowed {
		if value, ok := current[key]; ok {
			merged[key] = value
		}
	}
	for _, key := range allowed {
		value, exists := patch[key]
		if !exists {
			continue
		}
		if advanced && key == "cards" {
			merged[key] = mergeCards(merged[key], value)
			continue
		}
		merged[key] = value
	}
	return merged
}

func mergeCards(currentRaw, patchRaw any) map[string]any {
	merged := make(map[string]any)
	if current, ok := currentRaw.(map[string]any); ok {
		for key, value := range current {
			merged[key] = value
		}
	}
	if patch, ok := patchRaw.(map[string]any); ok {
		for key, value := range patch {
			if value == nil {
				delete(merged, key)
				continue
			}
			merged[key] = value
		}
	}
	return merged
}

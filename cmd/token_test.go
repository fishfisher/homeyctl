package cmd

import (
	"testing"
)

func TestScopePresets_Exist(t *testing.T) {
	expectedPresets := []string{"readonly", "control", "full"}

	for _, preset := range expectedPresets {
		if _, ok := scopePresets[preset]; !ok {
			t.Errorf("expected preset %q to exist", preset)
		}
	}
}

func TestScopePresets_ReadonlyHasNoWriteScopes(t *testing.T) {
	readonly := scopePresets["readonly"]

	writeScopes := []string{
		"homey.device.control",
		"homey.device",
		"homey.flow.start",
		"homey.flow",
		"homey.zone",
		"homey",
	}

	for _, scope := range readonly {
		for _, writeScope := range writeScopes {
			if scope == writeScope {
				t.Errorf("readonly preset should not contain write scope %q", writeScope)
			}
		}
	}
}

func TestScopePresets_ControlHasDeviceControl(t *testing.T) {
	control := scopePresets["control"]

	hasDeviceControl := false
	hasFlowStart := false

	for _, scope := range control {
		if scope == "homey.device.control" {
			hasDeviceControl = true
		}
		if scope == "homey.flow.start" {
			hasFlowStart = true
		}
	}

	if !hasDeviceControl {
		t.Error("control preset should contain homey.device.control")
	}
	if !hasFlowStart {
		t.Error("control preset should contain homey.flow.start")
	}
}

func TestScopePresets_FullHasHomeyScope(t *testing.T) {
	full := scopePresets["full"]

	hasHomey := false
	for _, scope := range full {
		if scope == "homey" {
			hasHomey = true
			break
		}
	}

	if !hasHomey {
		t.Error("full preset should contain homey scope")
	}
}

func TestAvailableScopes_ContainsCommonScopes(t *testing.T) {
	commonScopes := []string{
		"homey.device.readonly",
		"homey.device.control",
		"homey.flow.readonly",
		"homey.flow.start",
		"homey.zone.readonly",
	}

	for _, expected := range commonScopes {
		found := false
		for _, scope := range availableScopes {
			if scope == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("availableScopes should contain %q", expected)
		}
	}
}

func TestFormatScopes_Empty(t *testing.T) {
	result := formatScopes([]string{})
	if result != "-" {
		t.Errorf("formatScopes([]) = %q, want %q", result, "-")
	}
}

func TestFormatScopes_Single(t *testing.T) {
	result := formatScopes([]string{"homey.device.readonly"})
	if result != "homey.device.readonly" {
		t.Errorf("formatScopes([single]) = %q, want %q", result, "homey.device.readonly")
	}
}

func TestFormatScopes_Multiple(t *testing.T) {
	result := formatScopes([]string{"scope1", "scope2", "scope3"})
	if result != "scope1, scope2, scope3" {
		t.Errorf("formatScopes([3]) = %q, want comma-separated", result)
	}
}

func TestFormatScopes_ManyTruncates(t *testing.T) {
	scopes := []string{"scope1", "scope2", "scope3", "scope4", "scope5"}
	result := formatScopes(scopes)

	// Should show first scope + count
	if result != "scope1, +4 more" {
		t.Errorf("formatScopes([5]) = %q, want truncated format", result)
	}
}

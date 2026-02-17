package cmd

import (
	"testing"
)

func TestMaskToken_Empty(t *testing.T) {
	result := maskToken("")
	if result != "(not set)" {
		t.Errorf("maskToken(\"\") = %q, want %q", result, "(not set)")
	}
}

func TestMaskToken_Short(t *testing.T) {
	result := maskToken("short")
	if result != "short" {
		t.Errorf("maskToken(\"short\") = %q, want %q", result, "short")
	}
}

func TestMaskToken_Long(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwxyz"
	result := maskToken(token)
	expected := "abcdefgh...stuvwxyz" // first 8 + ... + last 8
	if result != expected {
		t.Errorf("maskToken(long) = %q, want %q", result, expected)
	}
}

func TestIsJSON_Default(t *testing.T) {
	// Save and restore the global jsonFlag
	oldFlag := jsonFlag
	defer func() { jsonFlag = oldFlag }()

	jsonFlag = false
	if isJSON() {
		t.Error("isJSON() should return false when --json flag is not set")
	}
}

func TestIsJSON_FlagSet(t *testing.T) {
	// Save and restore the global jsonFlag
	oldFlag := jsonFlag
	defer func() { jsonFlag = oldFlag }()

	jsonFlag = true
	if !isJSON() {
		t.Error("isJSON() should return true when --json flag is set")
	}
}

package cmd

import "testing"

func TestCapabilityWriteGuards(t *testing.T) {
	readOnly := false
	min, max := float64(0), float64(1)
	if err := validateCapabilityValue(Capability{Setable: &readOnly}, true); err == nil {
		t.Fatal("accepted read-only capability")
	}
	capability := Capability{Type: "number", Min: &min, Max: &max}
	for _, value := range []float64{-1, 2} {
		if err := validateCapabilityValue(capability, value); err == nil {
			t.Fatalf("accepted out-of-range %g", value)
		}
	}
	if err := validateCapabilityValue(capability, float64(0.5)); err != nil {
		t.Fatal(err)
	}
	if value := parseValue("12abc"); value != "12abc" {
		t.Fatalf("silently truncated value: %v", value)
	}
}

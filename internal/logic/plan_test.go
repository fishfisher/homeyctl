package logic

import (
	"fmt"
	"testing"
)

func TestParseValueRejectsSilentCoercions(t *testing.T) {
	for _, tc := range []struct{ kind, value string }{{"boolean", "ture"}, {"number", "12oops"}, {"number", "NaN"}, {"number", "+Inf"}, {"unknown", "true"}} {
		if _, err := ParseValue(tc.kind, tc.value); err == nil {
			t.Errorf("accepted %s %q", tc.kind, tc.value)
		}
	}
	value, err := ParseValue("string", "123")
	if err != nil || value != "123" {
		t.Fatal("string value was coerced")
	}
}

func TestPlanPreservesExistingValueAndRejectsCollisions(t *testing.T) {
	existing := map[string]Variable{"one": {ID: "one", Name: "Heating", Type: "boolean", Value: true}}
	requested := []Variable{{Name: "heating", Type: "boolean", Value: false}}
	plan, err := BuildPlan(requested, existing, "")
	if err != nil || len(plan.Create) != 0 || len(plan.Reuse) != 1 || plan.Reuse[0].Value != true {
		t.Fatalf("unsafe reuse: %#v %v", plan, err)
	}
	requested[0].Type = "string"
	requested[0].Value = "false"
	if _, err := BuildPlan(requested, existing, ""); err == nil {
		t.Fatal("accepted incompatible existing type")
	}
	if _, err := BuildPlan(append(requested, requested[0]), nil, ""); err == nil {
		t.Fatal("accepted duplicate names")
	}
}

func TestBatchApprovalBindsExactPlan(t *testing.T) {
	requested := make([]Variable, 10)
	for i := range requested {
		requested[i] = Variable{Name: fmt.Sprintf("AI.Test.%d", i), Type: "number", Value: float64(i), Purpose: "Count events"}
	}
	plan, err := BuildPlan(requested, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.CheckApproval(0, ""); err == nil {
		t.Fatal("unapproved batch accepted")
	}
	if err := plan.CheckApproval(9, plan.Approval); err == nil {
		t.Fatal("wrong count accepted")
	}
	if err := plan.CheckApproval(10, plan.Approval); err != nil {
		t.Fatal(err)
	}
	requested[0].Value = float64(99)
	changed, err := BuildPlan(requested, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := changed.CheckApproval(10, plan.Approval); err == nil {
		t.Fatal("stale approval accepted")
	}
}

func TestLargeBatchRequiresPurposeAndNamespace(t *testing.T) {
	requested := make([]Variable, 30)
	for i := range requested {
		requested[i] = Variable{Name: fmt.Sprintf("AI.Test.%d", i), Type: "boolean", Value: false, Purpose: "Explicit purpose"}
	}
	if _, err := BuildPlan(requested, nil, ""); err == nil {
		t.Fatal("30 variables without prefix accepted")
	}
	if _, err := BuildPlan(requested, nil, "AI.Test."); err != nil {
		t.Fatal(err)
	}
	requested[0].Purpose = ""
	if _, err := BuildPlan(requested, nil, "AI.Test."); err == nil {
		t.Fatal("missing purpose accepted")
	}
}

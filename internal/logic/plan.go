package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Variable struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   any    `json:"value"`
	Purpose string `json:"purpose,omitempty"`
}

type Plan struct {
	Create               []Variable `json:"create"`
	Reuse                []Variable `json:"reuse"`
	Approval             string     `json:"approval"`
	RequiresConfirmation bool       `json:"requiresConfirmation"`
	Prefix               string     `json:"prefix,omitempty"`
}

func ParseValue(kind, raw string) (any, error) {
	switch kind {
	case "string":
		return raw, nil
	case "boolean":
		switch strings.ToLower(raw) {
		case "true", "1", "yes":
			return true, nil
		case "false", "0", "no":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean %q (use true or false)", raw)
		}
	case "number":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid finite number %q", raw)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("invalid variable type %q (use string, number, boolean)", kind)
	}
}

func Validate(variable Variable) error {
	if strings.TrimSpace(variable.Name) == "" {
		return fmt.Errorf("variable name must not be empty")
	}
	valid := false
	switch variable.Type {
	case "string":
		_, valid = variable.Value.(string)
	case "boolean":
		_, valid = variable.Value.(bool)
	case "number":
		if n, ok := variable.Value.(float64); ok {
			valid = !math.IsNaN(n) && !math.IsInf(n, 0)
		}
	default:
		return fmt.Errorf("%q: unsupported type %q", variable.Name, variable.Type)
	}
	if !valid {
		return fmt.Errorf("%q: value must be a JSON %s", variable.Name, variable.Type)
	}
	return nil
}

// BuildPlan reuses matching variables without changing their current values.
func BuildPlan(requested []Variable, existing map[string]Variable, prefix string) (Plan, error) {
	plan := Plan{Create: []Variable{}, Reuse: []Variable{}, Prefix: prefix}
	if len(requested) == 0 {
		return plan, fmt.Errorf("manifest must contain at least one variable")
	}
	byName := map[string][]Variable{}
	for _, variable := range existing {
		key := strings.ToLower(variable.Name)
		byName[key] = append(byName[key], variable)
	}
	seen := map[string]bool{}
	for _, variable := range requested {
		if err := Validate(variable); err != nil {
			return plan, err
		}
		if variable.ID != "" {
			return plan, fmt.Errorf("%q: manifest entries must not specify an ID", variable.Name)
		}
		if prefix != "" && !strings.HasPrefix(variable.Name, prefix) {
			return plan, fmt.Errorf("%q must start with namespace prefix %q", variable.Name, prefix)
		}
		key := strings.ToLower(variable.Name)
		if seen[key] {
			return plan, fmt.Errorf("duplicate manifest name %q", variable.Name)
		}
		seen[key] = true
		matches := byName[key]
		if len(matches) > 1 {
			return plan, fmt.Errorf("existing variable name %q is ambiguous", variable.Name)
		}
		if len(matches) == 1 {
			if matches[0].Type != variable.Type {
				return plan, fmt.Errorf("%q already exists with type %s; requested %s", variable.Name, matches[0].Type, variable.Type)
			}
			plan.Reuse = append(plan.Reuse, matches[0])
		} else {
			plan.Create = append(plan.Create, variable)
		}
	}
	sort.Slice(plan.Create, func(i, j int) bool { return plan.Create[i].Name < plan.Create[j].Name })
	sort.Slice(plan.Reuse, func(i, j int) bool { return plan.Reuse[i].ID < plan.Reuse[j].ID })
	plan.RequiresConfirmation = len(plan.Create) >= 10
	if len(plan.Create) >= 10 {
		for _, variable := range plan.Create {
			if strings.TrimSpace(variable.Purpose) == "" {
				return plan, fmt.Errorf("%q: batches of 10+ new variables require a purpose for every new variable", variable.Name)
			}
		}
	}
	if len(plan.Create) >= 30 && strings.TrimSpace(prefix) == "" {
		return plan, fmt.Errorf("batches of 30+ new variables require --prefix and a reviewed namespace")
	}
	approvalPlan := plan
	approvalPlan.Reuse = append([]Variable(nil), plan.Reuse...)
	for i := range approvalPlan.Reuse {
		approvalPlan.Reuse[i].Value = nil
	}
	data, err := json.Marshal(approvalPlan)
	if err != nil {
		return plan, err
	}
	sum := sha256.Sum256(data)
	plan.Approval = hex.EncodeToString(sum[:])
	return plan, nil
}

func (p Plan) CheckApproval(count int, approval string) error {
	if p.RequiresConfirmation && (count != len(p.Create) || approval != p.Approval) {
		return fmt.Errorf("review the plan and obtain user approval for %d new variables; then use --confirm-count %d --approve %s", len(p.Create), len(p.Create), p.Approval)
	}
	return nil
}

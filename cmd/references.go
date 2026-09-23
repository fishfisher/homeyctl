package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	flowvalidation "github.com/fishfisher/homeyctl/internal/flow"
)

// usageIndex holds every flow document and HomeyScript once, so many lookups
// cost two flow requests plus one request per script. The flow list endpoints
// already return complete documents, cards included.
type usageIndex struct {
	flows   []indexedFlow
	scripts []indexedScript
	// scriptsErr is set when scripts could not be read (e.g. the HomeyScript
	// app is not installed); flows are still searched.
	scriptsErr error
}

type indexedFlow struct {
	id, name string
	advanced bool
	enabled  bool
	document map[string]any
}

type indexedScript struct {
	id, name, code string
}

// FlowUsage is one flow that references the target.
type FlowUsage struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Type       string                     `json:"type"`
	Enabled    bool                       `json:"enabled"`
	References []flowvalidation.Reference `json:"references"`
}

// ScriptUsage is one HomeyScript that mentions the target. Scripts usually
// look objects up by name, so a name match is reported but marked as such.
type ScriptUsage struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Match string `json:"match"` // "id" or "name"
}

type usage struct {
	Flows   []FlowUsage   `json:"flows"`
	Scripts []ScriptUsage `json:"scripts"`
}

func (u usage) empty() bool { return len(u.Flows) == 0 && len(u.Scripts) == 0 }

// loadUsageIndexFunc is a variable so tests can supply an index directly.
var loadUsageIndexFunc = loadUsageIndex

func loadUsageIndex() (*usageIndex, error) {
	index := &usageIndex{}
	for _, advanced := range []bool{false, true} {
		var data json.RawMessage
		var err error
		if advanced {
			data, err = apiClient.GetAdvancedFlows()
		} else {
			data, err = apiClient.GetFlows()
		}
		if err != nil {
			return nil, err
		}
		var documents map[string]map[string]any
		if err := json.Unmarshal(data, &documents); err != nil {
			return nil, fmt.Errorf("invalid flows response: %w", err)
		}
		for id, document := range documents {
			name, _ := document["name"].(string)
			enabled, _ := document["enabled"].(bool)
			index.flows = append(index.flows, indexedFlow{id: id, name: name, advanced: advanced, enabled: enabled, document: document})
		}
	}
	sort.Slice(index.flows, func(i, j int) bool {
		if index.flows[i].name != index.flows[j].name {
			return index.flows[i].name < index.flows[j].name
		}
		return index.flows[i].id < index.flows[j].id
	})

	data, err := apiClient.GetHomeyScripts()
	if err != nil {
		index.scriptsErr = err
		return index, nil
	}
	var list map[string]struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		index.scriptsErr = fmt.Errorf("invalid HomeyScript list: %w", err)
		return index, nil
	}
	for id, s := range list {
		if s.ID == "" {
			s.ID = id
		}
		raw, err := apiClient.GetHomeyScript(s.ID)
		if err != nil {
			index.scriptsErr = err
			continue
		}
		var full struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &full)
		index.scripts = append(index.scripts, indexedScript{id: s.ID, name: s.Name, code: full.Code})
	}
	sort.Slice(index.scripts, func(i, j int) bool { return index.scripts[i].name < index.scripts[j].name })
	return index, nil
}

// find reports where targetID is used. name is matched in script code only,
// and only when long enough not to match by accident. excludeFlow skips a
// flow's own document when the target is that flow.
func (ix *usageIndex) find(targetID, name, excludeFlow string) usage {
	result := usage{Flows: []FlowUsage{}, Scripts: []ScriptUsage{}}
	for _, f := range ix.flows {
		if f.id == excludeFlow {
			continue
		}
		refs := flowvalidation.FindReferences(f.document, f.advanced, targetID)
		if len(refs) == 0 {
			continue
		}
		result.Flows = append(result.Flows, FlowUsage{ID: f.id, Name: f.name, Type: flowKindShort(f.advanced), Enabled: f.enabled, References: refs})
	}
	lowerID := strings.ToLower(targetID)
	for _, s := range ix.scripts {
		code := strings.ToLower(s.code)
		switch {
		case strings.Contains(code, lowerID):
			result.Scripts = append(result.Scripts, ScriptUsage{ID: s.id, Name: s.name, Match: "id"})
		case len([]rune(name)) >= 4 && strings.Contains(code, strings.ToLower(name)):
			result.Scripts = append(result.Scripts, ScriptUsage{ID: s.id, Name: s.name, Match: "name"})
		}
	}
	return result
}

func flowKindShort(advanced bool) string {
	if advanced {
		return "advanced"
	}
	return "simple"
}

// describeCard is the table's short name for the card holding a reference;
// which fields matched is left to --json.
func describeCard(r flowvalidation.Reference) string {
	card := r.CardID
	if card == "" {
		return r.CardType
	}
	// Device IDs make card IDs long; the card's own verb is the readable part.
	if parts := strings.Split(card, ":"); len(parts) > 3 && parts[1] == "device" {
		return "device " + parts[len(parts)-1]
	}
	return card
}

func printUsage(u usage, scriptsErr error) {
	if len(u.Flows) == 0 {
		fmt.Println("No flows reference it.")
	} else {
		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Flow", "Type", "Enabled", "Where", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, f := range u.Flows {
			// One entry per card; a card can hold the ID in several fields.
			where := make([]string, 0, len(f.References))
			seenCard := map[string]bool{}
			for _, r := range f.References {
				if !seenCard[r.Card] {
					seenCard[r.Card] = true
					where = append(where, describeCard(r))
				}
			}
			enabled := "yes"
			if !f.Enabled {
				enabled = "no"
			}
			tbl.AddRow(f.Name, f.Type, enabled, strings.Join(where, ", "), f.ID)
		}
		tbl.Print()
	}
	for _, s := range u.Scripts {
		if s.Match == "id" {
			fmt.Printf("HomeyScript %q uses its ID.\n", s.Name)
		} else {
			fmt.Printf("HomeyScript %q mentions its name (possible use; check the script).\n", s.Name)
		}
	}
	if scriptsErr != nil {
		fmt.Printf("Note: HomeyScripts were not searched: %v\n", scriptsErr)
	}
}

var flowsFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find the flows and HomeyScripts that use a device, variable, or flow",
	Long: `List every flow that references a device, Logic variable, or other flow, and
where: trigger, condition or action card, droptoken, variable argument, or a
tag embedded in text. HomeyScripts are searched too; a script that only
mentions the name is reported as a possible use.

All flows are fetched in two requests, so this is fast even on large homes.
It is read-only.

Examples:
  homeyctl flows find --device "Stuekrok Ovn"
  homeyctl flows find --variable AI.Heating.Window.Stuekrok.Delta --json
  homeyctl flows find --flow "Good Morning"      # Flows that start or check it
  homeyctl flows find --id <any-uuid>`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		device, _ := cmd.Flags().GetString("device")
		variableArg, _ := cmd.Flags().GetString("variable")
		flowArg, _ := cmd.Flags().GetString("flow")
		rawID, _ := cmd.Flags().GetString("id")

		var kind, id, name, exclude string
		switch {
		case device != "":
			d, err := findDevice(device)
			if err != nil {
				return err
			}
			kind, id, name = "device", d.ID, d.Name
		case variableArg != "":
			variables, err := loadVariables()
			if err != nil {
				return err
			}
			v, err := resolveVariable(variables, variableArg)
			if err != nil {
				return err
			}
			kind, id, name = "variable", v.ID, v.Name
		case flowArg != "":
			f, err := findFlow(flowArg)
			if err != nil {
				return err
			}
			kind, id, name, exclude = "flow", f.ID, f.Name, f.ID
		case rawID != "":
			kind, id = "id", rawID
		default:
			return fmt.Errorf("give one of --device, --variable, --flow, or --id")
		}

		index, err := loadUsageIndexFunc()
		if err != nil {
			return err
		}
		u := index.find(id, name, exclude)
		if isJSON() {
			out := map[string]any{"target": map[string]any{"kind": kind, "id": id, "name": name}, "flows": u.Flows, "scripts": u.Scripts}
			if index.scriptsErr != nil {
				out["scriptsError"] = index.scriptsErr.Error()
			}
			return printJSONValue(out)
		}
		label := id
		if name != "" {
			label = fmt.Sprintf("%q (%s)", name, id)
		}
		color.New(color.Bold).Printf("Uses of %s %s\n\n", kind, label)
		printUsage(u, index.scriptsErr)
		return nil
	},
}

// VariableUsage is one row of variables usage.
type VariableUsage struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Flows   []FlowUsage   `json:"flows"`
	Scripts []ScriptUsage `json:"scripts"`
}

var varsUsageCmd = &cobra.Command{
	Use:   "usage [name-or-id]",
	Short: "Show which flows and HomeyScripts use Logic variables",
	Long: `Show where Logic variables are used, to review or clean up.

With a name or ID, report that variable. Otherwise report every variable, or
those whose name starts with --prefix. --unused keeps only variables that no
flow references and no HomeyScript mentions by ID or name: the candidates for
deletion. A variable a flow only writes still counts as used.

Examples:
  homeyctl variables usage AI.Heating.Window.Stuekrok.Delta
  homeyctl variables usage --prefix AI.             # Everything AI-built
  homeyctl variables usage --prefix AI. --unused    # Safe to remove`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prefix, _ := cmd.Flags().GetString("prefix")
		unusedOnly, _ := cmd.Flags().GetBool("unused")

		variables, err := loadVariables()
		if err != nil {
			return err
		}
		var selected []Variable
		if len(args) == 1 {
			v, err := resolveVariable(variables, args[0])
			if err != nil {
				return err
			}
			selected = []Variable{*v}
		} else {
			for _, v := range variables {
				if prefix == "" || strings.HasPrefix(strings.ToLower(v.Name), strings.ToLower(prefix)) {
					selected = append(selected, v)
				}
			}
			sort.Slice(selected, func(i, j int) bool { return selected[i].Name < selected[j].Name })
		}

		index, err := loadUsageIndexFunc()
		if err != nil {
			return err
		}
		rows := []VariableUsage{}
		for _, v := range selected {
			u := index.find(v.ID, v.Name, "")
			if unusedOnly && !u.empty() {
				continue
			}
			rows = append(rows, VariableUsage{ID: v.ID, Name: v.Name, Type: v.Type, Flows: u.Flows, Scripts: u.Scripts})
		}

		if isJSON() {
			out := map[string]any{"variables": rows}
			if index.scriptsErr != nil {
				out["scriptsError"] = index.scriptsErr.Error()
			}
			return printJSONValue(out)
		}
		if len(args) == 1 && len(rows) == 1 {
			color.New(color.Bold).Printf("Uses of variable %q (%s)\n\n", rows[0].Name, rows[0].ID)
			printUsage(usage{Flows: rows[0].Flows, Scripts: rows[0].Scripts}, index.scriptsErr)
			return nil
		}
		if len(rows) == 0 {
			if unusedOnly {
				fmt.Println("No unused variables.")
			} else {
				fmt.Println("No variables match.")
			}
			return nil
		}
		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Variable", "Type", "Flows", "Scripts", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, r := range rows {
			names := make([]string, 0, len(r.Flows))
			for _, f := range r.Flows {
				names = append(names, f.Name)
			}
			flows := "(unused)"
			if len(names) > 0 {
				flows = strings.Join(names, ", ")
			}
			scripts := ""
			for _, s := range r.Scripts {
				if scripts != "" {
					scripts += ", "
				}
				scripts += s.Name
				if s.Match == "name" {
					scripts += " (name?)"
				}
			}
			tbl.AddRow(r.Name, r.Type, flows, scripts, r.ID)
		}
		tbl.Print()
		if index.scriptsErr != nil {
			fmt.Printf("Note: HomeyScripts were not searched: %v\n", index.scriptsErr)
		}
		return nil
	},
}

func init() {
	flowsFindCmd.Flags().String("device", "", "Device name or ID")
	flowsFindCmd.Flags().String("variable", "", "Logic variable name or ID")
	flowsFindCmd.Flags().String("flow", "", "Flow name or ID")
	flowsFindCmd.Flags().String("id", "", "Any object ID (zone, user, folder, ...)")
	flowsFindCmd.MarkFlagsMutuallyExclusive("device", "variable", "flow", "id")
	flowsCmd.AddCommand(flowsFindCmd)

	varsUsageCmd.Flags().String("prefix", "", "Only variables whose name starts with this (case-insensitive)")
	varsUsageCmd.Flags().Bool("unused", false, "Only variables nothing references")
	varsCmd.AddCommand(varsUsageCmd)
}

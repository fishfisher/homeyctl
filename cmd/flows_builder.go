package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/fatih/color"
	flowvalidation "github.com/fishfisher/homeyctl/internal/flow"
	"github.com/spf13/cobra"
)

func printJSONValue(value any) error {
	return json.NewEncoder(os.Stdout).Encode(value)
}

func verifyFlowWrite(id string, expected map[string]any, advanced bool) error {
	actual, err := fetchFlow(id, id, advanced)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(actual.Raw, &document); err != nil {
		return err
	}
	for _, field := range []string{"name", "folder", "enabled"} {
		if value, exists := expected[field]; exists && !reflect.DeepEqual(value, document[field]) {
			return fmt.Errorf("field %q differs from requested value", field)
		}
	}
	if advanced {
		expectedCards, _ := expected["cards"].(map[string]any)
		actualCards, _ := document["cards"].(map[string]any)
		if len(expectedCards) != len(actualCards) {
			return fmt.Errorf("card count differs: expected %d, got %d", len(expectedCards), len(actualCards))
		}
		for id, card := range expectedCards {
			if !containsJSON(actualCards[id], card) {
				return fmt.Errorf("card %q differs from requested structure", id)
			}
		}
	} else {
		for _, field := range []string{"trigger", "conditions", "actions"} {
			if !containsJSON(document[field], expected[field]) {
				return fmt.Errorf("field %q differs from requested structure", field)
			}
		}
	}
	return nil
}

// Compare supplied fields while permitting extra server-generated metadata.
func containsJSON(actual, expected any) bool {
	switch expected := expected.(type) {
	case map[string]any:
		object, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range expected {
			if !containsJSON(object[key], value) {
				return false
			}
		}
		return true
	case []any:
		array, ok := actual.([]any)
		if !ok || len(array) != len(expected) {
			return false
		}
		for i, value := range expected {
			if !containsJSON(array[i], value) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

func readFlowDocument(path string) (map[string]any, error) {
	if path == "-" {
		path = "/dev/stdin"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if document == nil {
		return nil, fmt.Errorf("flow must be a JSON object")
	}
	return document, nil
}

func requireValidFlow(report flowvalidation.Report) error {
	if report.Valid() {
		return nil
	}
	messages := make([]string, 0, len(report.Errors))
	for _, issue := range report.Errors {
		messages = append(messages, issue.Path+": "+issue.Message)
	}
	return fmt.Errorf("flow validation failed:\n  %s", strings.Join(messages, "\n  "))
}

func printFlowPlan(action string, advanced bool, document map[string]any, report flowvalidation.Report, details map[string]any) error {
	if isJSON() {
		return printJSONValue(map[string]any{
			"action": action, "dryRun": true, "advanced": advanced,
			"payload": document, "validation": report, "details": details,
		})
	}

	kind := "flow"
	if advanced {
		kind = "advanced flow"
	}
	name, _ := document["name"].(string)
	color.New(color.Bold).Printf("Dry run: %s %s %q\n", action, kind, name)
	fmt.Println("Nothing was written to Homey.")
	fmt.Println()

	if id, ok := details["id"].(string); ok && id != "" {
		fmt.Printf("  Target:   %s\n", id)
	}
	if enabled, ok := document["enabled"].(bool); ok {
		fmt.Printf("  Enabled:  %t\n", enabled)
	}
	if ai, ok := details["ai"].(bool); ok && ai {
		folder, _ := details["aiFolder"].(string)
		fmt.Printf("  Folder:   %s (created if missing)\n", folder)
	}
	if advanced {
		cards, _ := document["cards"].(map[string]any)
		fmt.Printf("  Cards:    %d\n", len(cards))
	} else {
		conditions, _ := document["conditions"].([]any)
		actions, _ := document["actions"].([]any)
		fmt.Printf("  Conditions: %d\n", len(conditions))
		fmt.Printf("  Actions:    %d\n", len(actions))
	}
	fmt.Println()

	for _, issue := range append(report.Errors, report.Warnings...) {
		fmt.Printf("%s [%s] %s: %s\n", issue.Severity, issue.Code, issue.Path, issue.Message)
	}
	fmt.Printf("Validation: %d errors, %d warnings\n", len(report.Errors), len(report.Warnings))
	fmt.Println("Re-run without --dry-run to apply. Use --json for the full payload.")
	return nil
}

var flowsRestoreCmd = &cobra.Command{
	Use:   "restore <backup-file>",
	Short: "Restore a flow from a backup file written by this CLI",
	Long: `Restore a flow from a backup written by flows update, flows delete, or any
saved flows get output.

The target is taken from the backup's own id unless --to is given. The backup is
applied as the complete flow document, so cards, conditions, and actions added
since the backup are removed. The flow's current state is backed up first, and
the result is read back and compared before the command reports success.

A deleted flow cannot be restored in place because its id no longer exists;
recreate it with flows create and repair references from other flows.

Examples:
  homeyctl flows restore --dry-run ~/Library/Application\ Support/homeyctl/backups/My_Flow_2026....json
  homeyctl flows restore ~/Library/Application\ Support/homeyctl/backups/My_Flow_2026....json
  homeyctl flows restore backup.json --to "Other Flow"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		document, err := readFlowDocument(args[0])
		if err != nil {
			return err
		}

		target, _ := cmd.Flags().GetString("to")
		if target == "" {
			target, _ = document["id"].(string)
		}
		if target == "" {
			return fmt.Errorf("backup has no id; name the destination with --to <name-or-id>")
		}

		f, err := findFlow(target)
		if err != nil {
			return err
		}
		_, backupAdvanced := document["cards"]
		if backupAdvanced != f.Advanced {
			return fmt.Errorf("backup is %s but flow %q is %s; restore into a matching flow",
				flowKind(backupAdvanced), f.Name, flowKind(f.Advanced))
		}
		if err := flowvalidation.ValidatePatch(document, f.Advanced); err != nil {
			return fmt.Errorf("%s is not a flow backup: %w", args[0], err)
		}

		// A backup is already a complete document, so merging against nil yields
		// the full writable payload and drops anything added since.
		restored := flowvalidation.MergeUpdate(nil, document, f.Advanced)
		if !f.Advanced {
			normalizeSimpleFlow(restored)
		}
		report := flowvalidation.Validate(restored, f.Advanced)
		if err := requireValidFlow(report); err != nil {
			return err
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			return printFlowPlan("restore", f.Advanced, restored, report, map[string]any{"id": f.ID, "source": args[0]})
		}

		backupPath, err := backupFlow(f.Name, f.Raw)
		if err != nil {
			return fmt.Errorf("restore cancelled: backup failed: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backupPath)

		var result json.RawMessage
		if f.Advanced {
			result, err = apiClient.UpdateAdvancedFlow(f.ID, restored)
		} else {
			result, err = apiClient.UpdateFlow(f.ID, restored)
		}
		if err != nil {
			return fmt.Errorf("restore failed (backup: %s): %w", backupPath, err)
		}
		if err := verifyFlowWrite(f.ID, restored, f.Advanced); err != nil {
			return fmt.Errorf("restore applied but read-back differs (backup: %s); inspect before retrying: %w", backupPath, err)
		}
		if isJSON() {
			return printJSONValue(map[string]any{"flow": result, "backup": backupPath, "source": args[0]})
		}
		// Report the restored name; f.Name is the pre-restore state.
		restoredName, _ := restored["name"].(string)
		if restoredName == "" {
			restoredName = f.Name
		}
		color.Green("Restored %s: %s\n", flowKind(f.Advanced), restoredName)
		return nil
	},
}

func flowKind(advanced bool) string {
	if advanced {
		return "advanced flow"
	}
	return "flow"
}

// ensureAIFlowFolder only runs after validation and outside dry-run mode.
func ensureAIFlowFolder(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("AI folder name must not be empty")
	}
	data, err := apiClient.GetFlowFolders()
	if err != nil {
		return "", err
	}
	var folders map[string]FlowFolder
	if err := json.Unmarshal(data, &folders); err != nil {
		return "", err
	}
	var matches []string
	for _, folder := range folders {
		if strings.EqualFold(folder.Name, name) && folder.Parent == "" {
			matches = append(matches, folder.ID)
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple root folders named %q; resolve the duplicate folders first", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	created, err := apiClient.CreateFlowFolder(map[string]any{"name": name})
	if err != nil {
		return "", err
	}
	var folder FlowFolder
	if err := json.Unmarshal(created, &folder); err != nil {
		return "", err
	}
	if folder.ID == "" {
		return "", fmt.Errorf("folder creation returned no ID; inspect folders before retrying")
	}
	return folder.ID, nil
}

var flowsValidateCmd = &cobra.Command{
	Use:   "validate <json-file>",
	Short: "Validate flow JSON and graph structure without changing Homey",
	Long: `Validate a complete flow document. Advanced flows are detected by their cards field.
Checks card types, coordinates, output targets, ALL input references, and reachability.
Use --online to also check installed card IDs and Logic variable references.
This does not execute the flow or prove its intended behavior.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		document, err := readFlowDocument(args[0])
		if err != nil {
			return err
		}
		_, advanced := document["cards"]
		if !advanced {
			normalizeSimpleFlow(document)
		}
		report := flowvalidation.Validate(document, advanced)
		online, _ := cmd.Flags().GetBool("online")
		if online {
			catalog, err := loadFlowCatalog()
			if err != nil {
				return err
			}
			catalog.Validate(document, &report)
		}
		if isJSON() {
			if err := printJSONValue(report); err != nil {
				return err
			}
		} else {
			for _, issue := range append(report.Errors, report.Warnings...) {
				fmt.Printf("%s [%s] %s: %s\n", issue.Severity, issue.Code, issue.Path, issue.Message)
			}
			fmt.Printf("Validation: %d errors, %d warnings\n", len(report.Errors), len(report.Warnings))
		}
		if !report.Valid() {
			return fmt.Errorf("flow failed validation (%d errors)", len(report.Errors))
		}
		return nil
	},
}

func loadFlowCatalog() (*flowvalidation.Catalog, error) {
	catalog := &flowvalidation.Catalog{Cards: map[string]map[string]bool{}, Variables: map[string]flowvalidation.Variable{}}
	for _, kind := range []string{"trigger", "condition", "action"} {
		var data json.RawMessage
		var err error
		switch kind {
		case "trigger":
			data, err = apiClient.GetFlowTriggers()
		case "condition":
			data, err = apiClient.GetFlowConditions()
		case "action":
			data, err = apiClient.GetFlowActions()
		}
		if err != nil {
			return nil, fmt.Errorf("discover %s cards: %w", kind, err)
		}
		var cards []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &cards); err != nil {
			return nil, fmt.Errorf("decode %s cards: %w", kind, err)
		}
		catalog.Cards[kind] = map[string]bool{}
		for _, card := range cards {
			catalog.Cards[kind][card.ID] = true
		}
	}
	data, err := apiClient.GetVariables()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &catalog.Variables); err != nil {
		return nil, err
	}
	return catalog, nil
}

var flowsAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Read-only validation of all existing flows",
	Long: `Validate existing flows without changing anything.

Accepts the same filters as flows list, plus --problems to show only flows with
validation errors or warnings.

Examples:
  homeyctl flows audit --problems
  homeyctl flows audit --folder "AI Flows" --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filter, err := flowListFilterFromFlags(cmd)
		if err != nil {
			return err
		}
		problemsOnly, _ := cmd.Flags().GetBool("problems")
		type result struct {
			ID     string                `json:"id"`
			Name   string                `json:"name"`
			Report flowvalidation.Report `json:"validation"`
		}
		results := []result{}
		for _, advanced := range []bool{false, true} {
			var data json.RawMessage
			if advanced {
				data, err = apiClient.GetAdvancedFlows()
			} else {
				data, err = apiClient.GetFlows()
			}
			if err != nil {
				return err
			}
			var documents map[string]map[string]any
			if err := json.Unmarshal(data, &documents); err != nil {
				return err
			}
			for id, document := range documents {
				name, _ := document["name"].(string)
				folder, _ := document["folder"].(string)
				enabled, _ := document["enabled"].(bool)
				if !filter.keep(name, folderRef(folder), enabled) {
					continue
				}
				report := flowvalidation.Validate(document, advanced)
				if problemsOnly && len(report.Errors) == 0 && len(report.Warnings) == 0 {
					continue
				}
				results = append(results, result{ID: id, Name: name, Report: report})
			}
		}
		sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
		if isJSON() {
			return printJSONValue(results)
		}
		for _, result := range results {
			fmt.Printf("%s (%s): %d errors, %d warnings\n", result.Name, result.ID, len(result.Report.Errors), len(result.Report.Warnings))
		}
		return nil
	},
}

func init() {
	flowsCmd.AddCommand(flowsValidateCmd, flowsAuditCmd, flowsRestoreCmd)
	flowsValidateCmd.Flags().Bool("online", false, "Verify card IDs and Logic variables against this Homey (read-only)")
	flowsRestoreCmd.Flags().Bool("dry-run", false, "Validate and preview the restore without changing Homey")
	flowsRestoreCmd.Flags().String("to", "", "Restore into this flow instead of the backup's own id")
	// flowListFilterFromFlags reads --match through flowsMatchFilter, which is
	// bound to flows list; audit gets its own flag writing the same variable.
	flowsAuditCmd.Flags().StringVar(&flowsMatchFilter, "match", "", "Only flows whose name contains this (case-insensitive)")
	flowsAuditCmd.Flags().String("folder", "", "Only flows directly in this folder (name or ID)")
	flowsAuditCmd.Flags().Bool("enabled", false, "Only enabled flows")
	flowsAuditCmd.Flags().Bool("disabled", false, "Only disabled flows")
	flowsAuditCmd.Flags().Bool("problems", false, "Only flows with validation errors or warnings")
	flowsAuditCmd.MarkFlagsMutuallyExclusive("enabled", "disabled")
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	flowvalidation "github.com/fishfisher/homeyctl/internal/flow"
)

type Flow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Triggerable bool   `json:"triggerable"`
	Broken      bool   `json:"broken"`
}

type AdvancedFlow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Triggerable bool   `json:"triggerable"`
	Broken      bool   `json:"broken"`
}

var flowsCmd = &cobra.Command{
	Use:   "flows",
	Short: "Manage flows",
	Long:  `List, trigger, create, and delete Homey flows.`,
}

var flowsMatchFilter string

// foundFlow holds a resolved flow with its raw JSON and metadata.
type foundFlow struct {
	ID       string
	Name     string
	Advanced bool
	Raw      json.RawMessage
}

// findFlow looks up a flow by name or ID across both simple and advanced flows.
func findFlow(nameOrID string) (*foundFlow, error) {
	normalData, err := apiClient.GetFlows()
	if err != nil {
		return nil, err
	}
	advancedData, err := apiClient.GetAdvancedFlows()
	if err != nil {
		return nil, err
	}

	var normalFlows map[string]json.RawMessage
	if err := json.Unmarshal(normalData, &normalFlows); err != nil {
		return nil, fmt.Errorf("failed to parse flows: %w", err)
	}
	var advancedFlows map[string]json.RawMessage
	if err := json.Unmarshal(advancedData, &advancedFlows); err != nil {
		return nil, fmt.Errorf("failed to parse advanced flows: %w", err)
	}

	var matches []*foundFlow
	for _, raw := range normalFlows {
		var f Flow
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if f.ID == nameOrID {
			return fetchFlow(f.ID, f.Name, false)
		}
		if strings.EqualFold(f.Name, nameOrID) {
			matches = append(matches, &foundFlow{ID: f.ID, Name: f.Name, Advanced: false})
		}
	}

	for _, raw := range advancedFlows {
		var f AdvancedFlow
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if f.ID == nameOrID {
			return fetchFlow(f.ID, f.Name, true)
		}
		if strings.EqualFold(f.Name, nameOrID) {
			matches = append(matches, &foundFlow{ID: f.ID, Name: f.Name, Advanced: true})
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("flow not found: %s", nameOrID)
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, fmt.Sprintf("%s (%s)", match.ID, map[bool]string{true: "advanced", false: "simple"}[match.Advanced]))
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("flow name %q is ambiguous; use an ID: %s", nameOrID, strings.Join(ids, ", "))
	}
	return fetchFlow(matches[0].ID, matches[0].Name, matches[0].Advanced)
}

func fetchFlow(id, name string, advanced bool) (*foundFlow, error) {
	var (
		raw json.RawMessage
		err error
	)
	if advanced {
		raw, err = apiClient.GetAdvancedFlow(id)
	} else {
		raw, err = apiClient.GetFlow(id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch complete flow %q: %w", name, err)
	}
	return &foundFlow{ID: id, Name: name, Advanced: advanced, Raw: raw}, nil
}

// FlowListItem is the unified output format for flows
type FlowListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Enabled     bool   `json:"enabled"`
	Triggerable bool   `json:"triggerable"`
	Broken      bool   `json:"broken"`
}

var flowsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flows",
	Long: `List all flows, optionally filtered by name.

Examples:
  homeyctl flows list
  homeyctl flows list --match "night"
  homeyctl flows list --match "motion"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get both normal and advanced flows
		normalData, err := apiClient.GetFlows()
		if err != nil {
			return err
		}

		advancedData, err := apiClient.GetAdvancedFlows()
		if err != nil {
			return err
		}

		var normalFlows map[string]Flow
		var advancedFlows map[string]AdvancedFlow
		if err := json.Unmarshal(normalData, &normalFlows); err != nil {
			return fmt.Errorf("invalid flows response: %w", err)
		}
		if err := json.Unmarshal(advancedData, &advancedFlows); err != nil {
			return fmt.Errorf("invalid advanced flows response: %w", err)
		}

		// Build flat list with optional filtering
		allFlows := make([]FlowListItem, 0)
		for _, f := range normalFlows {
			if flowsMatchFilter == "" || strings.Contains(strings.ToLower(f.Name), strings.ToLower(flowsMatchFilter)) {
				allFlows = append(allFlows, FlowListItem{
					ID:          f.ID,
					Name:        f.Name,
					Type:        "simple",
					Enabled:     f.Enabled,
					Triggerable: f.Triggerable,
					Broken:      f.Broken,
				})
			}
		}
		for _, f := range advancedFlows {
			if flowsMatchFilter == "" || strings.Contains(strings.ToLower(f.Name), strings.ToLower(flowsMatchFilter)) {
				allFlows = append(allFlows, FlowListItem{
					ID:          f.ID,
					Name:        f.Name,
					Type:        "advanced",
					Enabled:     f.Enabled,
					Triggerable: f.Triggerable,
					Broken:      f.Broken,
				})
			}
		}

		sort.Slice(allFlows, func(i, j int) bool {
			if allFlows[i].Name == allFlows[j].Name {
				return allFlows[i].ID < allFlows[j].ID
			}
			return allFlows[i].Name < allFlows[j].Name
		})
		if isJSON() {
			out, _ := json.MarshalIndent(allFlows, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Type", "Enabled", "ID")
		tbl.WithHeaderFormatter(headerFmt)

		for _, f := range allFlows {
			enabled := "yes"
			if !f.Enabled {
				enabled = "no"
			}
			tbl.AddRow(f.Name, f.Type, enabled, f.ID)
		}

		tbl.Print()
		return nil
	},
}

var flowsTriggerCmd = &cobra.Command{
	Use:   "trigger <name-or-id>",
	Short: "Trigger a flow",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := findFlow(args[0])
		if err != nil {
			return err
		}

		if f.Advanced {
			if err := apiClient.TriggerAdvancedFlow(f.ID); err != nil {
				return err
			}
			color.Green("Triggered advanced flow: %s\n", f.Name)
		} else {
			if err := apiClient.TriggerFlow(f.ID); err != nil {
				return err
			}
			color.Green("Triggered flow: %s\n", f.Name)
		}
		return nil
	},
}

var flowsGetCmd = &cobra.Command{
	Use:   "get <name-or-id>",
	Short: "Get flow details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := findFlow(args[0])
		if err != nil {
			return err
		}
		outputJSON(f.Raw)
		return nil
	},
}

var flowsCreateCmd = &cobra.Command{
	Use:   "create <json-file>",
	Short: "Create a new flow",
	Long: `Create a new flow from a JSON file.

Use --advanced flag to create an advanced flow.

DISCOVERING IDs:
  - Device IDs:     homeyctl devices list
  - User IDs:       homeyctl users list
  - Flow card IDs:  homeyctl flows cards --type trigger|condition|action
  - Zone IDs:       homeyctl zones list

FLOW JSON STRUCTURE (group fields are auto-added if missing):
{
  "name": "Flow Name",
  "trigger": {
    "id": "homey:manager:presence:user_enter",
    "args": {
      "user": {"id": "<user-id>", "name": "<user-name>"}
    }
  },
  "conditions": [
    {
      "id": "homey:manager:logic:lt",
      "droptoken": "homey:device:<device-id>|measure_temperature",
      "args": {"comparator": 20}
    }
  ],
  "actions": [
    {"id": "homey:device:<device-id>:on", "args": {}},
    {"id": "homey:device:<device-id>:target_temperature_set", "args": {"target_temperature": 23}}
  ]
}

COMMON TRIGGER IDs:
  homey:manager:presence:user_enter      - A specific user came home
  homey:manager:presence:user_leave      - A user left home
  homey:manager:presence:first_user_enter - First person came home
  homey:manager:presence:last_user_left  - Last person left home

COMMON CONDITION IDs:
  homey:manager:logic:lt                 - Value is less than (use droptoken)
  homey:manager:logic:gt                 - Value is greater than (use droptoken)
  homey:manager:logic:eq                 - Value equals (use droptoken)
  homey:device:<id>:on                   - Device is on/off

COMMON ACTION IDs:
  homey:device:<id>:on                   - Turn device on
  homey:device:<id>:off                  - Turn device off
  homey:device:<id>:toggle               - Toggle device
  homey:device:<id>:dim                  - Set dim level (args: {"dim": 0.5})
  homey:device:<id>:target_temperature_set - Set temperature (args: {"target_temperature": 22})

DROPTOKENS (for logic conditions):
  Reference device capability values using: homey:device:<device-id>|<capability>
  Common capabilities: measure_temperature, measure_humidity, measure_power, onoff

Examples:
  homeyctl flows create flow.json
  homeyctl flows create --advanced advanced-flow.json
  cat flow.json | homeyctl flows create -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		advanced, _ := cmd.Flags().GetBool("advanced")

		var data []byte
		var err error

		if args[0] == "-" {
			data, err = os.ReadFile("/dev/stdin")
		} else {
			data, err = os.ReadFile(args[0])
		}
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var flow map[string]interface{}
		if err := json.Unmarshal(data, &flow); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		if flow == nil {
			return fmt.Errorf("flow must be a JSON object")
		}
		if _, exists := flow["cards"]; exists {
			advanced = true
		}
		if err := flowvalidation.ValidatePatch(flow, advanced); err != nil {
			return err
		}
		flow = flowvalidation.MergeUpdate(nil, flow, advanced)
		ai, _ := cmd.Flags().GetBool("ai")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		folderName, _ := cmd.Flags().GetString("ai-folder")
		if ai {
			flow["enabled"] = false
		}
		if !advanced {
			normalizeSimpleFlow(flow)
		}

		// Validate flow structure
		if err := validateFlow(flow, advanced); err != nil {
			return err
		}
		report := flowvalidation.Validate(flow, advanced)
		if err := requireValidFlow(report); err != nil {
			return err
		}
		if dryRun {
			return printFlowPlan("create", advanced, flow, report, map[string]any{"ai": ai, "aiFolder": folderName})
		}
		if ai {
			folderID, err := ensureAIFlowFolder(folderName)
			if err != nil {
				return err
			}
			flow["folder"] = folderID
		}

		// Normalize simple flow structure (add required group fields)
		if !advanced {
			normalizeSimpleFlow(flow)
		}

		var result json.RawMessage
		if advanced {
			result, err = apiClient.CreateAdvancedFlow(flow)
		} else {
			result, err = apiClient.CreateFlow(flow)
		}
		if err != nil {
			return err
		}

		var created struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(result, &created); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		if created.ID == "" {
			return fmt.Errorf("create response has no ID; inspect flows before retrying")
		}
		if err := verifyFlowWrite(created.ID, flow, advanced); err != nil {
			return fmt.Errorf("flow %s was created but verification failed; inspect before retrying: %w", created.ID, err)
		}
		if isJSON() {
			outputJSON(result)
			return nil
		}

		flowType := "flow"
		if advanced {
			flowType = "advanced flow"
		}
		color.Green("Created %s: %s (ID: %s)\n", flowType, created.Name, created.ID)
		return nil
	},
}

// validateFlow checks flow structure and returns helpful error messages
func validateFlow(flow map[string]interface{}, advanced bool) error {
	// Required: name
	name, ok := flow["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("validation error: 'name' is required and must be a string")
	}

	// Required: trigger (for simple flows)
	if !advanced {
		trigger, ok := flow["trigger"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("validation error: 'trigger' is required and must be an object")
		}

		triggerID, ok := trigger["id"].(string)
		if !ok || triggerID == "" {
			return fmt.Errorf("validation error: 'trigger.id' is required")
		}
		if !strings.HasPrefix(triggerID, "homey:") {
			return fmt.Errorf("validation error: 'trigger.id' must start with 'homey:' (got: %s)", triggerID)
		}
	}

	// Validate conditions
	if conditions, ok := flow["conditions"].([]interface{}); ok {
		for i, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				return fmt.Errorf("validation error: conditions[%d] must be an object", i)
			}

			condID, ok := cond["id"].(string)
			if !ok || condID == "" {
				return fmt.Errorf("validation error: conditions[%d].id is required", i)
			}

			// Check droptoken format for logic conditions
			if strings.Contains(condID, "logic:") {
				if droptoken, ok := cond["droptoken"].(string); ok {
					if strings.Contains(droptoken, "homey:device:") && !strings.Contains(droptoken, "|") {
						return fmt.Errorf("validation error: conditions[%d].droptoken uses wrong format. Use pipe (|) before capability: 'homey:device:<id>|<capability>'", i)
					}
				}
			}
		}
	}

	// Validate actions
	if actions, ok := flow["actions"].([]interface{}); ok {
		for i, a := range actions {
			action, ok := a.(map[string]interface{})
			if !ok {
				return fmt.Errorf("validation error: actions[%d] must be an object", i)
			}

			actionID, ok := action["id"].(string)
			if !ok || actionID == "" {
				return fmt.Errorf("validation error: actions[%d].id is required", i)
			}
			if !strings.HasPrefix(actionID, "homey:") {
				return fmt.Errorf("validation error: actions[%d].id must start with 'homey:' (got: %s)", i, actionID)
			}
		}
	}

	return nil
}

// normalizeSimpleFlow adds required fields that Homey expects
func normalizeSimpleFlow(flow map[string]interface{}) {
	for _, field := range []string{"conditions", "actions"} {
		if _, exists := flow[field]; !exists {
			flow[field] = []any{}
		}
	}
	// Add group to conditions
	if conditions, ok := flow["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if _, hasGroup := cond["group"]; !hasGroup {
					cond["group"] = "group1"
				}
				if _, hasInverted := cond["inverted"]; !hasInverted {
					cond["inverted"] = false
				}
			}
		}
	}

	// Add group to actions
	if actions, ok := flow["actions"].([]interface{}); ok {
		for _, a := range actions {
			if action, ok := a.(map[string]interface{}); ok {
				if _, hasGroup := action["group"]; !hasGroup {
					action["group"] = "then"
				}
			}
		}
	}
}

// backupFlow saves the current flow state to a JSON file and returns the path.
var backupFlow = saveBackup

func saveBackup(name string, rawData json.RawMessage) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to find config dir: %w", err)
	}

	backupDir := filepath.Join(configDir, "homeyctl", "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Sanitize name for filename
	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if len(safeName) > 80 {
		safeName = safeName[:80]
	}
	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	filename := fmt.Sprintf("%s_%s.json", safeName, timestamp)
	path := filepath.Join(backupDir, filename)

	// Pretty-print the JSON
	var pretty json.RawMessage
	if err := json.Unmarshal(rawData, &pretty); err != nil {
		return "", err
	}
	formatted, _ := json.MarshalIndent(pretty, "", "  ")

	if err := os.WriteFile(path, formatted, 0o600); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return path, nil
}

var flowsUpdateCmd = &cobra.Command{
	Use:   "update <name-or-id> [json-file]",
	Short: "Update an existing flow",
	Long: `Update an existing flow from a JSON file, inline JSON, or stdin.

IMPORTANT: This does a partial/merge update - only fields you include will be
changed. Fields you omit keep their existing values. To remove conditions or
actions, explicitly set them to an empty array: "conditions": []

The current flow is ALWAYS backed up before applying changes. If backup fails,
the update is cancelled. Use --dry-run to preview without writing anything.
Backups are stored in ~/Library/Application Support/homeyctl/backups/.

Examples:
  # Full update workflow
  homeyctl flows get "My Flow" > flow.json
  # Edit flow.json
  homeyctl flows update "My Flow" flow.json

  # Preview the merged result without writing anything
  homeyctl flows update --dry-run "My Flow" flow.json

  # Inline JSON via --data
  homeyctl flows update "My Flow" --data '{"name": "New Name"}'

  # Pipe from stdin
  echo '{"name": "New Name"}' | homeyctl flows update "My Flow" -

  # Remove all conditions
  echo '{"conditions": []}' | homeyctl flows update "My Flow" -`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		dataFlag, _ := cmd.Flags().GetString("data")

		var data []byte
		var err error

		switch {
		case dataFlag != "":
			data = []byte(dataFlag)
		case len(args) == 2 && args[1] == "-":
			data, err = os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
		case len(args) == 2:
			data, err = os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
		default:
			return fmt.Errorf("provide update JSON via --data, a file path, or - for stdin")
		}

		var flow map[string]interface{}
		if err := json.Unmarshal(data, &flow); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		f, err := findFlow(nameOrID)
		if err != nil {
			return err
		}

		var current map[string]any
		if err := json.Unmarshal(f.Raw, &current); err != nil {
			return err
		}
		if err := flowvalidation.ValidatePatch(flow, f.Advanced); err != nil {
			return err
		}
		replaceCards, _ := cmd.Flags().GetBool("replace-cards")
		if replaceCards && f.Advanced {
			cards, ok := flow["cards"].(map[string]any)
			if !ok {
				return fmt.Errorf("--replace-cards requires a complete cards object")
			}
			if currentCards, ok := current["cards"].(map[string]any); ok {
				for id := range currentCards {
					if _, exists := cards[id]; !exists {
						cards[id] = nil
					}
				}
			}
		}
		merged := flowvalidation.MergeUpdate(current, flow, f.Advanced)
		if !f.Advanced {
			normalizeSimpleFlow(merged)
		}
		report := flowvalidation.Validate(merged, f.Advanced)
		if err := requireValidFlow(report); err != nil {
			return err
		}
		if dryRun {
			return printFlowPlan("update", f.Advanced, merged, report, map[string]any{"id": f.ID, "before": current})
		}
		backupPath, err := backupFlow(f.Name, f.Raw)
		if err != nil {
			return fmt.Errorf("update cancelled: backup failed: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backupPath)
		// Send the complete validated graph. Null entries are a CLI patch
		// convention and must never be forwarded as invalid Homey cards.
		payload := merged
		var result json.RawMessage
		if f.Advanced {
			result, err = apiClient.UpdateAdvancedFlow(f.ID, payload)
		} else {
			result, err = apiClient.UpdateFlow(f.ID, payload)
		}
		if err != nil {
			return fmt.Errorf("update failed (backup: %s): %w", backupPath, err)
		}
		if err := verifyFlowWrite(f.ID, merged, f.Advanced); err != nil {
			return fmt.Errorf("update applied but read-back differs (backup: %s); inspect before retrying: %w", backupPath, err)
		}
		if isJSON() {
			return printJSONValue(map[string]any{"flow": result, "backup": backupPath})
		}
		color.Green("Updated flow: %s\n", f.Name)
		return nil
	},
}

var flowsDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a flow",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			return fmt.Errorf("deleting a flow requires --force; its current state will be backed up first")
		}
		f, err := findFlow(args[0])
		if err != nil {
			return err
		}

		backupPath, err := backupFlow(f.Name, f.Raw)
		if err != nil {
			return fmt.Errorf("delete cancelled: backup failed: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backupPath)
		if f.Advanced {
			if err := apiClient.DeleteAdvancedFlow(f.ID); err != nil {
				return err
			}
			color.Green("Deleted advanced flow: %s\n", f.Name)
		} else {
			if err := apiClient.DeleteFlow(f.ID); err != nil {
				return err
			}
			color.Green("Deleted flow: %s\n", f.Name)
		}
		return nil
	},
}

var flowsCardsCmd = &cobra.Command{
	Use:   "cards",
	Short: "List available flow cards",
	Long: `List available flow cards (triggers, conditions, actions).

Use this to discover card IDs for creating flows.

Card types:
  trigger   - Events that start a flow (user arrives, time, device changes)
  condition - Checks that must pass (temperature < X, device is on)
  action    - Things to do (turn on device, send notification)

The card ID format is: homey:<owner>:<card-name>
  - homey:manager:presence:user_enter (system trigger)
  - homey:device:<device-id>:on (device action)
  - homey:manager:logic:lt (logic condition)

Examples:
  homeyctl flows cards --type trigger
  homeyctl flows cards --type trigger | jq '.[] | select(.id | contains("presence"))'
  homeyctl flows cards --type action | jq '.[] | select(.id | contains("<device-id>"))'
  homeyctl flows cards --type condition | jq '.[] | select(.id | contains("logic"))'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cardType, _ := cmd.Flags().GetString("type")
		filter, _ := cmd.Flags().GetString("filter")

		var data json.RawMessage
		var err error

		switch cardType {
		case "trigger":
			data, err = apiClient.GetFlowTriggers()
		case "condition":
			data, err = apiClient.GetFlowConditions()
		case "action":
			data, err = apiClient.GetFlowActions()
		default:
			return fmt.Errorf("invalid card type: %s (use: trigger, condition, action)", cardType)
		}

		if err != nil {
			return err
		}

		var cards []map[string]any
		if err := json.Unmarshal(data, &cards); err != nil {
			return err
		}
		filtered := make([]map[string]any, 0, len(cards))
		for _, card := range cards {
			id, _ := card["id"].(string)
			title, _ := card["title"].(string)
			if filter == "" || strings.Contains(strings.ToLower(id), strings.ToLower(filter)) || strings.Contains(strings.ToLower(title), strings.ToLower(filter)) {
				filtered = append(filtered, card)
			}
		}
		if isJSON() {
			return printJSONValue(filtered)
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Title", "ID")
		tbl.WithHeaderFormatter(headerFmt)

		for _, card := range filtered {
			tbl.AddRow(card["title"], card["id"])
		}
		tbl.Print()
		return nil
	},
}

var flowsAutocompleteCmd = &cobra.Command{
	Use:   "autocomplete <card-id> <arg-name>",
	Short: "Query autocomplete values for a flow card argument",
	Long: `Query the dynamic autocomplete list for a flow card argument.

Some flow card arguments (like Sonos favourites) are populated dynamically
by the app at runtime. Use this command to discover valid values.

The card ID is the full ID including the owner URI prefix
(e.g., homey:device:<device-id>:<card-name>). The ownerUri is derived
automatically from the card ID.

Examples:
  # List all Sonos favourites
  homeyctl flows autocomplete "homey:device:55fb5f52-462d-4175-ba96-17c54c40a96b:cloud_play_sonos_favorite" favorite

  # Search for a specific favourite
  homeyctl flows autocomplete "homey:device:55fb5f52-462d-4175-ba96-17c54c40a96b:cloud_play_sonos_favorite" favorite --query "golden"

  # Query a condition card
  homeyctl flows autocomplete "homey:device:abc123:some_condition" mode --type condition`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID := args[0]
		argName := args[1]
		query, _ := cmd.Flags().GetString("query")
		cardTypeFlag, _ := cmd.Flags().GetString("type")

		// Map short type names to API card type
		var cardType string
		switch cardTypeFlag {
		case "action":
			cardType = "flowcardaction"
		case "condition":
			cardType = "flowcardcondition"
		case "trigger":
			cardType = "flowcardtrigger"
		default:
			return fmt.Errorf("invalid card type: %s (use: action, condition, trigger)", cardTypeFlag)
		}

		// Derive ownerUri from card ID: everything except the last :segment
		lastColon := strings.LastIndex(cardID, ":")
		if lastColon == -1 {
			return fmt.Errorf("invalid card ID format: %s (expected format like homey:device:<id>:<card>)", cardID)
		}
		ownerURI := cardID[:lastColon]

		data, err := apiClient.GetFlowCardAutocomplete(cardType, ownerURI, cardID, argName, query)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Description", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, item := range items {
			tbl.AddRow(item.Name, item.Description, item.ID)
		}
		tbl.Print()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(flowsCmd)
	flowsCmd.AddCommand(flowsListCmd)
	flowsListCmd.Flags().StringVar(&flowsMatchFilter, "match", "", "Filter flows by name (case-insensitive)")
	flowsCmd.AddCommand(flowsGetCmd)
	flowsCmd.AddCommand(flowsCreateCmd)
	flowsCmd.AddCommand(flowsUpdateCmd)
	flowsCmd.AddCommand(flowsTriggerCmd)
	flowsCmd.AddCommand(flowsDeleteCmd)
	flowsCmd.AddCommand(flowsCardsCmd)
	flowsCmd.AddCommand(flowsAutocompleteCmd)

	flowsUpdateCmd.Flags().Bool("backup", false, "Compatibility flag; backups are now always created")
	// Kept so existing scripts keep working, but hidden from help so it no
	// longer reads as something the user has to opt into.
	_ = flowsUpdateCmd.Flags().MarkDeprecated("backup", "backups are always created")
	flowsUpdateCmd.Flags().Bool("dry-run", false, "Validate and preview the merged flow without changes")
	flowsUpdateCmd.Flags().Bool("replace-cards", false, "Replace the entire advanced graph, including removal of omitted cards")
	flowsUpdateCmd.Flags().String("data", "", "Inline JSON data for the update")
	flowsCreateCmd.Flags().Bool("advanced", false, "Create an advanced flow")
	flowsCreateCmd.Flags().Bool("ai", false, "Create disabled in the AI Flows review folder")
	flowsCreateCmd.Flags().String("ai-folder", "AI Flows", "Review folder for --ai (created if missing)")
	flowsCreateCmd.Flags().Bool("dry-run", false, "Validate and preview without creating a flow or folder")
	flowsDeleteCmd.Flags().Bool("force", false, "Confirm deletion (automatic backup required)")
	flowsCardsCmd.Flags().String("type", "action", "Card type: trigger, condition, action")
	flowsCardsCmd.Flags().String("filter", "", "Filter cards by name or ID")

	flowsAutocompleteCmd.Flags().String("query", "", "Search query to filter results")
	flowsAutocompleteCmd.Flags().String("type", "action", "Card type: action, condition, trigger")
}

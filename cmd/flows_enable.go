package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	flowvalidation "github.com/fishfisher/homeyctl/internal/flow"
)

var flowsEnableCmd = &cobra.Command{
	Use:   "enable <name-or-id>",
	Short: "Enable a flow (e.g. an AI draft after review)",
	Long: `Enable a flow so its trigger starts firing.

This is the last step of the review workflow: flows created with --ai start
disabled, and enabling one authorizes its physical effects. The command
therefore prints what the flow triggers on and what it does, and only applies
with --yes. Without --yes it shows that summary and changes nothing.

The current flow is backed up first, and the result is read back.

Examples:
  homeyctl flows list --folder "AI Flows" --disabled   # Drafts awaiting review
  homeyctl flows enable "Night lights"                 # Show what enabling does
  homeyctl flows enable "Night lights" --yes           # Enable it`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setFlowEnabled(cmd, args[0], true)
	},
}

var flowsDisableCmd = &cobra.Command{
	Use:   "disable <name-or-id>",
	Short: "Disable a flow",
	Long: `Disable a flow so its trigger stops firing.

The current flow is backed up first, and the result is read back. Disabling is
allowed even for a flow that fails validation, so a broken flow can always be
switched off.

Examples:
  homeyctl flows disable "Night lights"
  homeyctl flows disable "Night lights" --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setFlowEnabled(cmd, args[0], false)
	},
}

func setFlowEnabled(cmd *cobra.Command, nameOrID string, enabled bool) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes := false
	if enabled {
		yes, _ = cmd.Flags().GetBool("yes")
	}
	verb := "disable"
	if enabled {
		verb = "enable"
	}

	f, err := findFlow(nameOrID)
	if err != nil {
		return err
	}
	var current map[string]any
	if err := json.Unmarshal(f.Raw, &current); err != nil {
		return err
	}

	if was, _ := current["enabled"].(bool); was == enabled {
		if isJSON() {
			return printJSONValue(map[string]any{"id": f.ID, "name": f.Name, "enabled": enabled, "changed": false})
		}
		fmt.Printf("%s %q is already %sd; nothing to do.\n", capitalize(flowKind(f.Advanced)), f.Name, verb)
		return nil
	}

	patch := map[string]any{"enabled": enabled}
	merged := flowvalidation.MergeUpdate(current, patch, f.Advanced)
	if !f.Advanced {
		normalizeSimpleFlow(merged)
	}
	report := flowvalidation.Validate(merged, f.Advanced)
	// Only enabling needs a valid flow: switching a broken flow off must work.
	if enabled {
		if err := requireValidFlow(report); err != nil {
			return err
		}
	}

	effects := describeFlowEffects(merged, f.Advanced)
	if dryRun || (enabled && !yes) {
		effects = effects.withDeviceNames(deviceNames())
		if isJSON() {
			return printJSONValue(map[string]any{
				"action": verb, "dryRun": true, "id": f.ID, "name": f.Name,
				"advanced": f.Advanced, "effects": effects, "validation": report,
				"requiresConfirmation": enabled,
			})
		}
		printFlowEffects(verb, f, effects)
		if dryRun {
			fmt.Println("Dry run: nothing was written to Homey.")
			return nil
		}
		fmt.Println("Nothing was changed. Enabling lets this flow act on its own from now on.")
		return fmt.Errorf("re-run with --yes to enable %q", f.Name)
	}

	backupPath, err := backupFlow(f.Name, f.Raw)
	if err != nil {
		return fmt.Errorf("%s cancelled: backup failed: %w", verb, err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backupPath)

	if f.Advanced {
		_, err = apiClient.UpdateAdvancedFlow(f.ID, merged)
	} else {
		_, err = apiClient.UpdateFlow(f.ID, merged)
	}
	if err != nil {
		return fmt.Errorf("%s failed (backup: %s): %w", verb, backupPath, err)
	}
	if err := verifyFlowWrite(f.ID, merged, f.Advanced); err != nil {
		return fmt.Errorf("%s applied but read-back differs (backup: %s); inspect before retrying: %w", verb, backupPath, err)
	}

	if isJSON() {
		return printJSONValue(map[string]any{"id": f.ID, "name": f.Name, "enabled": enabled, "changed": true, "backup": backupPath})
	}
	color.Green("%sd %s: %s\n", capitalize(verb), flowKind(f.Advanced), f.Name)
	return nil
}

// flowEffects is a readable summary of what a flow reacts to and does, built
// from card IDs such as "homey:device:<id>:on".
type flowEffects struct {
	Triggers   []string `json:"triggers"`
	Conditions []string `json:"conditions"`
	Actions    []string `json:"actions"`
}

func describeFlowEffects(document map[string]any, advanced bool) flowEffects {
	effects := flowEffects{Triggers: []string{}, Conditions: []string{}, Actions: []string{}}
	cardID := func(card any) string {
		m, _ := card.(map[string]any)
		id, _ := m["id"].(string)
		return id
	}

	if !advanced {
		if id := cardID(document["trigger"]); id != "" {
			effects.Triggers = append(effects.Triggers, id)
		}
		conditions, _ := document["conditions"].([]any)
		for _, c := range conditions {
			if id := cardID(c); id != "" {
				effects.Conditions = append(effects.Conditions, id)
			}
		}
		actions, _ := document["actions"].([]any)
		for _, a := range actions {
			if id := cardID(a); id != "" {
				effects.Actions = append(effects.Actions, id)
			}
		}
		return effects
	}

	cards, _ := document["cards"].(map[string]any)
	for _, raw := range cards {
		card, _ := raw.(map[string]any)
		cardType, _ := card["type"].(string)
		id := cardID(card)
		switch cardType {
		case "trigger":
			effects.Triggers = append(effects.Triggers, id)
		case "start":
			effects.Triggers = append(effects.Triggers, "start (manual / triggered by another flow)")
		case "condition":
			effects.Conditions = append(effects.Conditions, id)
		case "action":
			effects.Actions = append(effects.Actions, id)
		case "delay":
			effects.Actions = append(effects.Actions, "delay")
		}
	}
	sort.Strings(effects.Triggers)
	sort.Strings(effects.Conditions)
	sort.Strings(effects.Actions)
	return effects
}

// withDeviceNames rewrites "homey:device:<id>:<card>" as "<device name>: <card>"
// so a reviewer sees which lamp or speaker a card drives. A card whose device
// no longer exists is marked as such. When the lookup failed altogether, every
// card keeps its raw ID.
func (e flowEffects) withDeviceNames(names map[string]string) flowEffects {
	if len(names) == 0 {
		return e
	}
	rename := func(items []string) []string {
		out := make([]string, len(items))
		for i, item := range items {
			out[i] = item
			rest, ok := strings.CutPrefix(item, "homey:device:")
			if !ok {
				continue
			}
			id, card, ok := strings.Cut(rest, ":")
			if !ok {
				continue
			}
			if name, known := names[id]; known {
				out[i] = name + ": " + card
			} else {
				// Worth seeing before enabling: the card will not work.
				out[i] = item + " (device not found)"
			}
		}
		return out
	}
	return flowEffects{Triggers: rename(e.Triggers), Conditions: rename(e.Conditions), Actions: rename(e.Actions)}
}

// deviceNames maps device IDs to names. It is best-effort: the summary is
// still correct, only less readable, when the lookup fails.
func deviceNames() map[string]string {
	data, err := apiClient.GetDevices()
	if err != nil {
		return nil
	}
	var devices map[string]Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil
	}
	names := make(map[string]string, len(devices))
	for id, d := range devices {
		names[id] = d.Name
	}
	return names
}

func printFlowEffects(verb string, f *foundFlow, e flowEffects) {
	color.New(color.Bold).Printf("%s %s %q\n", capitalize(verb), flowKind(f.Advanced), f.Name)
	fmt.Printf("  ID: %s\n\n", f.ID)
	section := func(title string, items []string) {
		fmt.Printf("  %s:\n", title)
		if len(items) == 0 {
			fmt.Println("    (none)")
		}
		for _, item := range items {
			fmt.Printf("    - %s\n", item)
		}
	}
	section("Triggers", e.Triggers)
	section("Conditions", e.Conditions)
	section("Actions", e.Actions)
	fmt.Println()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func init() {
	flowsEnableCmd.Flags().Bool("yes", false, "Confirm enabling; without it the effects are shown and nothing changes")
	flowsEnableCmd.Flags().Bool("dry-run", false, "Show what enabling would do without changing anything")
	flowsDisableCmd.Flags().Bool("dry-run", false, "Show what disabling would do without changing anything")
	flowsCmd.AddCommand(flowsEnableCmd)
	flowsCmd.AddCommand(flowsDisableCmd)
}

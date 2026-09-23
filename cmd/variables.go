package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/fishfisher/homeyctl/internal/logic"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

type Variable = logic.Variable

var varsCmd = &cobra.Command{Use: "variables", Aliases: []string{"vars", "var"}, Short: "Manage Logic variables and review batch creation plans"}

func loadVariables() (map[string]Variable, error) {
	data, err := apiClient.GetVariables()
	if err != nil {
		return nil, err
	}
	var variables map[string]Variable
	if err := json.Unmarshal(data, &variables); err != nil {
		return nil, fmt.Errorf("invalid variables response: %w", err)
	}
	return variables, nil
}

func resolveVariable(variables map[string]Variable, nameOrID string) (*Variable, error) {
	if variable, ok := variables[nameOrID]; ok {
		return &variable, nil
	}
	var matches []Variable
	for _, variable := range variables {
		if variable.ID == nameOrID {
			return &variable, nil
		}
		if strings.EqualFold(variable.Name, nameOrID) {
			matches = append(matches, variable)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("variable not found: %s", nameOrID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("variable name %q is ambiguous; use its ID", nameOrID)
	}
	return &matches[0], nil
}

var varsListCmd = &cobra.Command{Use: "list", Short: "List all variables", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
	variables, err := loadVariables()
	if err != nil {
		return err
	}
	if isJSON() {
		return printJSONValue(variables)
	}
	headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
	tbl := table.New("Name", "Type", "Value", "ID")
	tbl.WithHeaderFormatter(headerFmt)
	for _, variable := range variables {
		tbl.AddRow(variable.Name, variable.Type, variable.Value, variable.ID)
	}
	tbl.Print()
	return nil
}}

// printVariable renders a single variable, honouring the --json flag like every
// other command group.
func printVariable(variable Variable) error {
	if isJSON() {
		return printJSONValue(variable)
	}
	color.New(color.Bold).Println(variable.Name)
	fmt.Printf("  Type:  %s\n", variable.Type)
	fmt.Printf("  Value: %v\n", variable.Value)
	fmt.Printf("  ID:    %s\n", variable.ID)
	return nil
}

var varsGetCmd = &cobra.Command{Use: "get <name-or-id>", Short: "Get a Logic variable", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	variables, err := loadVariables()
	if err != nil {
		return err
	}
	variable, err := resolveVariable(variables, args[0])
	if err != nil {
		return err
	}
	return printVariable(*variable)
}}

var varsSetCmd = &cobra.Command{Use: "set <name-or-id> <value>", Short: "Set a variable value (may trigger flows)", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
	variables, err := loadVariables()
	if err != nil {
		return err
	}
	variable, err := resolveVariable(variables, args[0])
	if err != nil {
		return err
	}
	value, err := logic.ParseValue(variable.Type, args[1])
	if err != nil {
		return err
	}
	before, err := json.Marshal(variable)
	if err != nil {
		return err
	}
	backup, err := backupFlow("variable-"+variable.Name, before)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backup)
	if err := apiClient.SetVariable(variable.ID, value); err != nil {
		return err
	}
	variable.Value = value
	return printVariable(*variable)
}}

var varsCreateCmd = &cobra.Command{Use: "create <name> <type> <value>", Short: "Create one variable; reject name collisions", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
	value, err := logic.ParseValue(args[1], args[2])
	if err != nil {
		return err
	}
	variable := Variable{Name: args[0], Type: args[1], Value: value}
	if err := logic.Validate(variable); err != nil {
		return err
	}
	variables, err := loadVariables()
	if err != nil {
		return err
	}
	for _, existing := range variables {
		if strings.EqualFold(existing.Name, variable.Name) {
			return fmt.Errorf("variable %q already exists (%s); reuse it or choose another name", variable.Name, existing.ID)
		}
	}
	result, err := apiClient.CreateVariable(variable.Name, variable.Type, variable.Value)
	if err != nil {
		return err
	}
	if isJSON() {
		outputJSON(result)
		return nil
	}
	var created Variable
	if err := json.Unmarshal(result, &created); err != nil {
		return fmt.Errorf("invalid create response: %w", err)
	}
	color.Green("Created variable: %s\n", created.Name)
	return printVariable(created)
}}

var varsDeleteCmd = &cobra.Command{Use: "delete <name-or-id>", Short: "Delete a variable after backup; requires --force", Long: `Delete a Logic variable. The variable is backed up first.

Deletion is refused while any flow references the variable or a HomeyScript
mentions it, because those would break silently: a recreated variable gets a
new ID. The refusal lists them; see also variables usage. Pass
--allow-referenced only when you have decided to break or repair them.`, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		return fmt.Errorf("deletion requires --force; check first with: homeyctl variables usage %q", args[0])
	}
	allowReferenced, _ := cmd.Flags().GetBool("allow-referenced")
	variables, err := loadVariables()
	if err != nil {
		return err
	}
	variable, err := resolveVariable(variables, args[0])
	if err != nil {
		return err
	}
	if !allowReferenced {
		index, err := loadUsageIndexFunc()
		if err != nil {
			return fmt.Errorf("delete cancelled: could not check which flows use %q: %w (pass --allow-referenced to delete without checking)", variable.Name, err)
		}
		if u := index.find(variable.ID, variable.Name, ""); !u.empty() {
			fmt.Fprintf(cmd.ErrOrStderr(), "%q is still in use:\n", variable.Name)
			for _, f := range u.Flows {
				fmt.Fprintf(cmd.ErrOrStderr(), "  flow %q (%s)\n", f.Name, f.ID)
			}
			for _, sc := range u.Scripts {
				fmt.Fprintf(cmd.ErrOrStderr(), "  HomeyScript %q (matches its %s)\n", sc.Name, sc.Match)
			}
			return fmt.Errorf("delete cancelled: %q is used by %d flow(s) and %d HomeyScript(s); pass --allow-referenced to delete anyway", variable.Name, len(u.Flows), len(u.Scripts))
		}
	}
	before, err := json.Marshal(variable)
	if err != nil {
		return err
	}
	backup, err := backupFlow("variable-"+variable.Name, before)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Backup: %s\n", backup)
	if err := apiClient.DeleteVariable(variable.ID); err != nil {
		return err
	}
	if isJSON() {
		return printJSONValue(map[string]any{"deleted": variable, "backup": backup})
	}
	color.Green("Deleted variable: %s (%s)\n", variable.Name, variable.ID)
	fmt.Printf("  Backup: %s\n", backup)
	return nil
}}

var varsBatchCmd = &cobra.Command{
	Use: "batch <manifest.json>", Short: "Plan variable creation; use --apply only after reviewing",
	Long: `Read an array of {name,type,value,purpose} objects. Default: read-only plan.
Matching existing names/types are reused, and their current values are preserved.
10+ new variables require purpose fields, --confirm-count, and the plan's --approve hash.
30+ also require --prefix. Agents must obtain user approval before supplying those flags.
Every apply saves a receipt. On partial failure, inspect the receipt and re-plan;
do not blindly retry or automatically delete variables already created.`,
	Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if path == "-" {
			path = "/dev/stdin"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var requested []Variable
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&requested); err != nil {
			return fmt.Errorf("invalid manifest: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return fmt.Errorf("manifest must contain exactly one JSON array")
		}
		variables, err := loadVariables()
		if err != nil {
			return err
		}
		prefix, _ := cmd.Flags().GetString("prefix")
		plan, err := logic.BuildPlan(requested, variables, prefix)
		if err != nil {
			return err
		}
		apply, _ := cmd.Flags().GetBool("apply")
		if !apply {
			if isJSON() {
				return printJSONValue(plan)
			}
			return printVariablePlan(plan)
		}
		count, _ := cmd.Flags().GetInt("confirm-count")
		approval, _ := cmd.Flags().GetString("approve")
		if err := plan.CheckApproval(count, approval); err != nil {
			return err
		}
		receipt := map[string]any{"plan": plan, "created": []Variable{}, "status": "in_progress"}
		initial, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		receiptPath, err := backupFlow("variables-batch", initial)
		if err != nil {
			return fmt.Errorf("receipt failed: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Receipt: %s\n", receiptPath)
		created := []Variable{}
		for _, variable := range plan.Create {
			receipt["pending"] = variable.Name
			if err := saveVariableReceipt(receiptPath, receipt); err != nil {
				return err
			}
			result, err := apiClient.CreateVariable(variable.Name, variable.Type, variable.Value)
			if err != nil {
				return fmt.Errorf("batch stopped after %d creations; inspect receipt %s and refresh variables before retrying: %w", len(created), receiptPath, err)
			}
			var actual Variable
			if err := json.Unmarshal(result, &actual); err != nil || actual.ID == "" {
				return fmt.Errorf("creation response is uncertain; inspect receipt %s and refresh variables before retrying", receiptPath)
			}
			created = append(created, actual)
			receipt["created"] = created
			delete(receipt, "pending")
			if err := saveVariableReceipt(receiptPath, receipt); err != nil {
				return err
			}
		}
		receipt["status"] = "complete"
		if err := saveVariableReceipt(receiptPath, receipt); err != nil {
			return err
		}
		receipt["receipt"] = receiptPath
		if isJSON() {
			return printJSONValue(receipt)
		}
		color.Green("Created %d variable(s)\n", len(created))
		for _, variable := range created {
			fmt.Printf("  %-30s %-8s %v\n", variable.Name, variable.Type, variable.Value)
		}
		fmt.Printf("  Receipt: %s\n", receiptPath)
		return nil
	},
}

// printVariablePlan renders the read-only plan a human is expected to approve.
func printVariablePlan(plan logic.Plan) error {
	color.New(color.Bold).Printf("Variable plan: %d to create, %d reused\n", len(plan.Create), len(plan.Reuse))
	if len(plan.Create) > 0 {
		fmt.Println()
		color.New(color.FgCyan, color.Underline).Println("Create")
		for _, variable := range plan.Create {
			fmt.Printf("  %-30s %-8s %v", variable.Name, variable.Type, variable.Value)
			if variable.Purpose != "" {
				fmt.Printf("  — %s", variable.Purpose)
			}
			fmt.Println()
		}
	}
	if len(plan.Reuse) > 0 {
		fmt.Println()
		color.New(color.FgCyan, color.Underline).Println("Reuse (values left untouched)")
		for _, variable := range plan.Reuse {
			fmt.Printf("  %-30s %-8s %s\n", variable.Name, variable.Type, variable.ID)
		}
	}
	fmt.Println()
	if plan.RequiresConfirmation {
		color.Yellow("This batch needs explicit user approval before --apply.\n")
		fmt.Printf("  After approval: --apply --confirm-count %d --approve %s\n", len(plan.Create), plan.Approval)
		if plan.Prefix != "" {
			fmt.Printf("  Namespace prefix: %s\n", plan.Prefix)
		}
		return nil
	}
	fmt.Println("Nothing has been written. Re-run with --apply to create these variables.")
	return nil
}

func saveVariableReceipt(path string, receipt map[string]any) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".receipt-*")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func init() {
	rootCmd.AddCommand(varsCmd)
	varsCmd.AddCommand(varsListCmd, varsGetCmd, varsSetCmd, varsCreateCmd, varsDeleteCmd, varsBatchCmd)
	varsDeleteCmd.Flags().Bool("force", false, "Confirm deletion")
	varsDeleteCmd.Flags().Bool("allow-referenced", false, "Delete even though flows or HomeyScripts still use the variable")
	varsBatchCmd.Flags().Bool("apply", false, "Apply a reviewed plan (default: read-only)")
	varsBatchCmd.Flags().Int("confirm-count", 0, "Explicitly approved number of NEW variables for batches of 10+")
	varsBatchCmd.Flags().String("approve", "", "Hash of the reviewed plan for batches of 10+")
	varsBatchCmd.Flags().String("prefix", "", "Required name prefix (mandatory for 30+ new variables)")
}

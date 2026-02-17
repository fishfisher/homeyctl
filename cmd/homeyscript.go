package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

type HomeyScript struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Version      int     `json:"version"`
	LastExecuted *string `json:"lastExecuted"`
}

var homeyscriptCmd = &cobra.Command{
	Use:     "homeyscript",
	Aliases: []string{"hs"},
	Short:   "Manage HomeyScript scripts",
	Long:    `List, view, create, update, delete, and run HomeyScript scripts.`,
}

func findHomeyScript(nameOrID string) (*HomeyScript, error) {
	data, err := apiClient.GetHomeyScripts()
	if err != nil {
		return nil, err
	}

	var scripts map[string]HomeyScript
	if err := json.Unmarshal(data, &scripts); err != nil {
		return nil, fmt.Errorf("failed to parse scripts: %w", err)
	}

	for _, s := range scripts {
		if s.ID == nameOrID || strings.EqualFold(s.Name, nameOrID) {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("script not found: %s", nameOrID)
}

var homeyscriptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scripts",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetHomeyScripts()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var scripts map[string]HomeyScript
		if err := json.Unmarshal(data, &scripts); err != nil {
			return fmt.Errorf("failed to parse scripts: %w", err)
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Version", "Last Executed", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, s := range scripts {
			lastExec := "-"
			if s.LastExecuted != nil {
				lastExec = *s.LastExecuted
			}
			tbl.AddRow(s.Name, s.Version, lastExec, s.ID)
		}
		tbl.Print()
		return nil
	},
}

var homeyscriptGetCmd = &cobra.Command{
	Use:   "get <name-or-id>",
	Short: "Get script details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := findHomeyScript(args[0])
		if err != nil {
			return err
		}

		if isJSON() {
			data, err := apiClient.GetHomeyScript(script.ID)
			if err != nil {
				return err
			}
			outputJSON(data)
			return nil
		}

		fmt.Printf("Name:          %s\n", script.Name)
		fmt.Printf("ID:            %s\n", script.ID)
		fmt.Printf("Version:       %d\n", script.Version)
		lastExec := "-"
		if script.LastExecuted != nil {
			lastExec = *script.LastExecuted
		}
		fmt.Printf("Last Executed: %s\n", lastExec)
		fmt.Printf("\n--- Code ---\n%s\n", script.Code)
		return nil
	},
}

var homeyscriptCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new script",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		code, _ := cmd.Flags().GetString("code")
		file, _ := cmd.Flags().GetString("file")

		if file == "" && code == "" {
			return fmt.Errorf("provide script code via --file or --code")
		}

		if file != "" {
			contents, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			code = string(contents)
		}

		data, err := apiClient.CreateHomeyScript(name, code)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var created HomeyScript
		if err := json.Unmarshal(data, &created); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		color.Green("Created script: %s (%s)\n", created.Name, created.ID)
		return nil
	},
}

var homeyscriptUpdateCmd = &cobra.Command{
	Use:   "update <name-or-id>",
	Short: "Update an existing script",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := findHomeyScript(args[0])
		if err != nil {
			return err
		}

		newName := script.Name
		if n, _ := cmd.Flags().GetString("name"); n != "" {
			newName = n
		}

		code := script.Code
		if file, _ := cmd.Flags().GetString("file"); file != "" {
			contents, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			code = string(contents)
		} else if c, _ := cmd.Flags().GetString("code"); c != "" {
			code = c
		}

		data, err := apiClient.UpdateHomeyScript(script.ID, newName, code, script.Version)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		color.Green("Updated script: %s\n", newName)
		return nil
	},
}

var homeyscriptDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a script",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := findHomeyScript(args[0])
		if err != nil {
			return err
		}

		if err := apiClient.DeleteHomeyScript(script.ID); err != nil {
			return err
		}

		color.Green("Deleted script: %s\n", script.Name)
		return nil
	},
}

var homeyscriptRunCmd = &cobra.Command{
	Use:   "run <name-or-id> [args...]",
	Short: "Run a script",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := findHomeyScript(args[0])
		if err != nil {
			return err
		}

		var scriptArgs []string
		if len(args) > 1 {
			scriptArgs = args[1:]
		}

		data, err := apiClient.RunHomeyScript(script.ID, scriptArgs)
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		// Print the run result
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(homeyscriptCmd)
	homeyscriptCmd.AddCommand(homeyscriptListCmd)
	homeyscriptCmd.AddCommand(homeyscriptGetCmd)
	homeyscriptCmd.AddCommand(homeyscriptCreateCmd)
	homeyscriptCmd.AddCommand(homeyscriptUpdateCmd)
	homeyscriptCmd.AddCommand(homeyscriptDeleteCmd)
	homeyscriptCmd.AddCommand(homeyscriptRunCmd)

	homeyscriptCreateCmd.Flags().String("file", "", "Path to .js file containing script code")
	homeyscriptCreateCmd.Flags().String("code", "", "Inline script code")

	homeyscriptUpdateCmd.Flags().String("file", "", "Path to .js file containing new script code")
	homeyscriptUpdateCmd.Flags().String("code", "", "Inline script code")
	homeyscriptUpdateCmd.Flags().String("name", "", "New name for the script")
}

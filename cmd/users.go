package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Present bool   `json:"present"`
	Asleep  bool   `json:"asleep"`
}

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users",
	Long:  `List and view Homey users.`,
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetUsers()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var users map[string]User
		if err := json.Unmarshal(data, &users); err != nil {
			return fmt.Errorf("failed to parse users: %w", err)
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Name", "Role", "Present", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, u := range users {
			present := "no"
			if u.Present {
				present = "yes"
			}
			tbl.AddRow(u.Name, u.Role, present, u.ID)
		}
		tbl.Print()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(usersListCmd)
}

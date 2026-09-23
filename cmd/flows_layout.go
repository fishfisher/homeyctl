package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	flowvalidation "github.com/fishfisher/homeyctl/internal/flow"
)

var flowsLayoutCmd = &cobra.Command{
	Use:   "layout <flow-file>",
	Short: "Arrange an Advanced Flow's cards so they do not overlap (offline)",
	Long: `Rewrite the x/y positions of an Advanced Flow document so it reads left to
right without overlapping cards. Nothing is sent to Homey.

Columns follow execution order, chains stay level, and branches stack in port
order: true above false above error. Each note stays with the card it was
placed nearest to: a note up to 400 wide sits directly above that card, a wider
note goes in a band above the graph. Only positions change.

Card sizes are estimated (Homey does not store them), so check the result in
the editor. flows validate warns about overlaps using the same estimates.

Works on a draft or on the output of flows get. Apply a laid-out existing flow
with flows update, which backs it up first.

Examples:
  homeyctl flows layout draft.json --write
  homeyctl flows layout draft.json > laid-out.json
  homeyctl flows get <id> --json > flow.json && homeyctl flows layout flow.json --write
  homeyctl flows update <id> flow.json --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")
		document, err := readFlowDocument(args[0])
		if err != nil {
			return err
		}
		if _, ok := document["cards"].(map[string]any); !ok {
			return fmt.Errorf("%s is not an Advanced Flow; only Advanced Flows have card positions", args[0])
		}

		result := flowvalidation.Layout(document)
		out, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		out = append(out, '\n')

		summary := fmt.Sprintf("Laid out %d cards and %d notes in %d columns; %d estimated overlaps remain.",
			result.Cards, result.Notes, result.Columns, result.Overlaps)
		if write {
			info, err := os.Stat(args[0])
			if err != nil {
				return err
			}
			if err := os.WriteFile(args[0], out, info.Mode().Perm()); err != nil {
				return err
			}
			if isJSON() {
				return printJSONValue(map[string]any{"file": args[0], "layout": result})
			}
			fmt.Println(summary)
			fmt.Printf("Wrote %s\n", args[0])
			return nil
		}
		// The document goes to stdout so it can be redirected; the summary
		// goes to stderr so it never corrupts that JSON.
		fmt.Fprintln(cmd.ErrOrStderr(), summary)
		_, err = os.Stdout.Write(out)
		return err
	},
}

func init() {
	flowsLayoutCmd.Flags().Bool("write", false, "Rewrite the file in place instead of printing the result")
	flowsCmd.AddCommand(flowsLayoutCmd)
}

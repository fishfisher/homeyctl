package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var snapshotIncludeFlows bool

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Get a snapshot of Homey state",
	Long: `Get status, zones, and devices in one call.

Useful for AI assistants and scripts that need a complete overview.

Examples:
  homeyctl snapshot
  homeyctl snapshot --include-flows
  homeyctl snapshot --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get system status
		systemData, err := apiClient.GetSystem()
		if err != nil {
			return fmt.Errorf("failed to get system status: %w", err)
		}

		// Get zones
		zonesData, err := apiClient.GetZones()
		if err != nil {
			return fmt.Errorf("failed to get zones: %w", err)
		}

		// Get devices
		devicesData, err := apiClient.GetDevices()
		if err != nil {
			return fmt.Errorf("failed to get devices: %w", err)
		}

		// Parse for counting
		var zones map[string]interface{}
		var devices map[string]interface{}
		if err := json.Unmarshal(zonesData, &zones); err != nil {
			return fmt.Errorf("invalid zones response: %w", err)
		}
		if err := json.Unmarshal(devicesData, &devices); err != nil {
			return fmt.Errorf("invalid devices response: %w", err)
		}

		snapshot := map[string]json.RawMessage{
			"system":  systemData,
			"zones":   zonesData,
			"devices": devicesData,
		}

		// Optionally include flows
		if snapshotIncludeFlows {
			flowsData, err := apiClient.GetFlows()
			if err != nil {
				return fmt.Errorf("failed to get flows: %w", err)
			}
			snapshot["flows"] = flowsData

			advFlowsData, err := apiClient.GetAdvancedFlows()
			if err != nil {
				return fmt.Errorf("failed to get advanced flows: %w", err)
			}
			snapshot["advancedFlows"] = advFlowsData
		}

		if isJSON() {
			out, err := json.MarshalIndent(snapshot, "", "  ")
			if err != nil {
				return fmt.Errorf("invalid snapshot response: %w", err)
			}
			fmt.Println(string(out))
			return nil
		}

		var system map[string]interface{}
		if err := json.Unmarshal(systemData, &system); err != nil {
			return fmt.Errorf("invalid system response: %w", err)
		}

		color.New(color.Bold).Println("Homey Snapshot")
		fmt.Println("==============")
		fmt.Printf("Model:     %v\n", system["homeyModelName"])
		fmt.Printf("Version:   %v\n", system["homeyVersion"])
		fmt.Printf("Platform:  %v (v%v)\n", system["homeyPlatform"], system["homeyPlatformVersion"])
		fmt.Printf("Hostname:  %v\n", system["hostname"])
		fmt.Printf("Zones:     %d\n", len(zones))
		fmt.Printf("Devices:   %d\n", len(devices))

		if snapshotIncludeFlows {
			var flows, advFlows map[string]interface{}
			if err := json.Unmarshal(snapshot["flows"], &flows); err != nil {
				return fmt.Errorf("invalid flows response: %w", err)
			}
			if err := json.Unmarshal(snapshot["advancedFlows"], &advFlows); err != nil {
				return fmt.Errorf("invalid advanced flows response: %w", err)
			}
			fmt.Printf("Flows:     %d\n", len(flows))
			fmt.Printf("Advanced:  %d\n", len(advFlows))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.Flags().BoolVar(&snapshotIncludeFlows, "include-flows", false, "Include flows in snapshot")
}

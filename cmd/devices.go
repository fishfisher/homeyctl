package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

// Device represents a Homey device
type Device struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Class           string                `json:"class"`
	Zone            string                `json:"zone"`
	CapabilitiesObj map[string]Capability `json:"capabilitiesObj"`
}

// Capability represents a device capability
type Capability struct {
	ID      string      `json:"id"`
	Value   interface{} `json:"value"`
	Title   string      `json:"title"`
	Type    string      `json:"type,omitempty"`
	Setable *bool       `json:"setable,omitempty"`
	Min     *float64    `json:"min,omitempty"`
	Max     *float64    `json:"max,omitempty"`
	Values  []struct {
		ID string `json:"id"`
	} `json:"values,omitempty"`
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Manage devices",
	Long:  `List, view, control, and manage Homey devices.`,
}

var devicesMatchFilter string

// findDevice finds a device by name or ID from the list of all devices
func findDevice(nameOrID string) (*Device, error) {
	data, err := apiClient.GetDevices()
	if err != nil {
		return nil, err
	}

	var devices map[string]Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil, fmt.Errorf("failed to parse devices: %w", err)
	}

	var matches []Device
	for _, d := range devices {
		if d.ID == nameOrID {
			return &d, nil
		}
		if strings.EqualFold(d.Name, nameOrID) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("device not found: %s", nameOrID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("device name %q is ambiguous; use its ID", nameOrID)
	}
	return &matches[0], nil
}

var devicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	Long: `List all devices, optionally filtered by name or zone.

--zone includes every zone beneath it, so a floor lists the devices in its
rooms. The table then gains a Zone column.

Examples:
  homeyctl devices list
  homeyctl devices list --match "kitchen"
  homeyctl devices list --zone "Hovedetasje"
  homeyctl devices list --zone "Stue" --match "lamp"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zoneName, _ := cmd.Flags().GetString("zone")
		var zoneNames map[string]string // zone ID → name, only with --zone
		var inZone map[string]bool      // the zone and everything beneath it
		if zoneName != "" {
			zone, err := findZone(zoneName)
			if err != nil {
				return err
			}
			zones, err := loadZones()
			if err != nil {
				return err
			}
			inZone = zoneSubtree(zones, zone.ID)
			zoneNames = make(map[string]string, len(zones))
			for id, z := range zones {
				zoneNames[id] = z.Name
			}
		}

		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		var filtered []Device
		for _, d := range devices {
			if devicesMatchFilter != "" && !strings.Contains(strings.ToLower(d.Name), strings.ToLower(devicesMatchFilter)) {
				continue
			}
			if inZone != nil && !inZone[d.Zone] {
				continue
			}
			filtered = append(filtered, d)
		}
		// Map iteration order is random; keep output stable across runs.
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].Name == filtered[j].Name {
				return filtered[i].ID < filtered[j].ID
			}
			return filtered[i].Name < filtered[j].Name
		})

		if isJSON() {
			out, _ := json.MarshalIndent(filtered, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		if zoneNames != nil {
			tbl := table.New("Name", "Class", "Zone", "ID")
			tbl.WithHeaderFormatter(headerFmt)
			for _, d := range filtered {
				tbl.AddRow(d.Name, d.Class, zoneNames[d.Zone], d.ID)
			}
			tbl.Print()
			return nil
		}
		tbl := table.New("Name", "Class", "ID")
		tbl.WithHeaderFormatter(headerFmt)
		for _, d := range filtered {
			tbl.AddRow(d.Name, d.Class, d.ID)
		}
		tbl.Print()
		return nil
	},
}

func loadZones() (map[string]Zone, error) {
	data, err := apiClient.GetZones()
	if err != nil {
		return nil, err
	}
	var zones map[string]Zone
	if err := json.Unmarshal(data, &zones); err != nil {
		return nil, fmt.Errorf("failed to parse zones: %w", err)
	}
	return zones, nil
}

// zoneSubtree returns root and every zone beneath it. It guards against a
// parent cycle, which would otherwise loop forever on malformed data.
func zoneSubtree(zones map[string]Zone, root string) map[string]bool {
	children := make(map[string][]string, len(zones))
	for id, z := range zones {
		if z.Parent != "" {
			children[z.Parent] = append(children[z.Parent], id)
		}
	}
	in := map[string]bool{root: true}
	queue := []string{root}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, child := range children[id] {
			if !in[child] {
				in[child] = true
				queue = append(queue, child)
			}
		}
	}
	return in
}

var devicesGetCmd = &cobra.Command{
	Use:   "get <name-or-id>",
	Short: "Get device details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := findDevice(args[0])
		if err != nil {
			return err
		}

		if isJSON() {
			data, err := apiClient.GetDevice(device.ID)
			if err != nil {
				return err
			}
			outputJSON(data)
			return nil
		}

		color.New(color.Bold).Println(device.Name)
		fmt.Printf("  Class: %s\n", device.Class)
		fmt.Printf("  ID:    %s\n", device.ID)
		fmt.Println("\n  Capabilities:")

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Capability", "Value")
		tbl.WithHeaderFormatter(headerFmt)
		for _, cap := range device.CapabilitiesObj {
			tbl.AddRow(cap.ID, cap.Value)
		}
		tbl.Print()
		return nil
	},
}

var devicesValuesCmd = &cobra.Command{
	Use:   "values <name-or-id>",
	Short: "Get all capability values for a device",
	Long: `Get all current capability values for a device.

Useful for multi-sensors and devices with many capabilities.

Examples:
  homeyctl devices values "PultLED"
  homeyctl devices values "Multisensor 6"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := findDevice(args[0])
		if err != nil {
			return err
		}

		if isJSON() {
			// JSON output - just the values
			values := make(map[string]interface{})
			for _, cap := range device.CapabilitiesObj {
				values[cap.ID] = cap.Value
			}
			out, _ := json.MarshalIndent(map[string]interface{}{
				"id":     device.ID,
				"name":   device.Name,
				"values": values,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		color.New(color.Bold).Printf("Values for %s:\n\n", device.Name)
		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Capability", "Value")
		tbl.WithHeaderFormatter(headerFmt)
		for _, cap := range device.CapabilitiesObj {
			tbl.AddRow(cap.ID, cap.Value)
		}
		tbl.Print()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.AddCommand(devicesListCmd)
	devicesListCmd.Flags().StringVar(&devicesMatchFilter, "match", "", "Filter devices by name (case-insensitive)")
	devicesListCmd.Flags().String("zone", "", "Only devices in this zone or any zone beneath it (name or ID)")
	devicesCmd.AddCommand(devicesGetCmd)
	devicesCmd.AddCommand(devicesValuesCmd)
}

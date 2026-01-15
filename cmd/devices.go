package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type Device struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Class           string                `json:"class"`
	Zone            string                `json:"zone"`
	CapabilitiesObj map[string]Capability `json:"capabilitiesObj"`
}

type Capability struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
	Title string      `json:"title"`
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Manage devices",
	Long:  `List, view, control, and delete Homey devices.`,
}

var devicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		if isTableFormat() {
			var devices map[string]Device
			if err := json.Unmarshal(data, &devices); err != nil {
				return fmt.Errorf("failed to parse devices: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tCLASS\tID")
			fmt.Fprintln(w, "----\t-----\t--")
			for _, d := range devices {
				fmt.Fprintf(w, "%s\t%s\t%s\n", d.Name, d.Class, d.ID)
			}
			w.Flush()
			return nil
		}

		outputJSON(data)
		return nil
	},
}

var devicesGetCmd = &cobra.Command{
	Use:   "get <name-or-id>",
	Short: "Get device details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		// First get all devices to find by name
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		// Find device by name or ID
		var device *Device
		for _, d := range devices {
			if d.ID == nameOrID || strings.EqualFold(d.Name, nameOrID) {
				device = &d
				break
			}
		}

		if device == nil {
			return fmt.Errorf("device not found: %s", nameOrID)
		}

		if isTableFormat() {
			fmt.Printf("Name:  %s\n", device.Name)
			fmt.Printf("Class: %s\n", device.Class)
			fmt.Printf("ID:    %s\n", device.ID)
			fmt.Println("\nCapabilities:")

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  CAPABILITY\tVALUE")
			fmt.Fprintln(w, "  ----------\t-----")
			for _, cap := range device.CapabilitiesObj {
				fmt.Fprintf(w, "  %s\t%v\n", cap.ID, cap.Value)
			}
			w.Flush()
			return nil
		}

		out, _ := json.MarshalIndent(device, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var devicesSetCmd = &cobra.Command{
	Use:   "set <name-or-id> <capability> <value>",
	Short: "Set device capability",
	Long: `Set a device capability value.

Examples:
  homeyctl devices set "PultLED" onoff true
  homeyctl devices set "PultLED" dim 0.5
  homeyctl devices set "Aksels rom" target_temperature 22`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		capability := args[1]
		valueStr := args[2]

		// Find device ID
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		var deviceID string
		for _, d := range devices {
			if d.ID == nameOrID || strings.EqualFold(d.Name, nameOrID) {
				deviceID = d.ID
				break
			}
		}

		if deviceID == "" {
			return fmt.Errorf("device not found: %s", nameOrID)
		}

		// Parse value
		var value interface{}
		if valueStr == "true" {
			value = true
		} else if valueStr == "false" {
			value = false
		} else {
			// Try as number
			var num float64
			if _, err := fmt.Sscanf(valueStr, "%f", &num); err == nil {
				value = num
			} else {
				value = valueStr
			}
		}

		if err := apiClient.SetCapability(deviceID, capability, value); err != nil {
			return err
		}

		fmt.Printf("Set %s.%s = %v\n", nameOrID, capability, value)
		return nil
	},
}

var devicesSetSettingCmd = &cobra.Command{
	Use:   "set-setting <name-or-id> <setting-key> <value>",
	Short: "Set device setting",
	Long: `Set a device setting value.

Device settings are different from capabilities - they configure device behavior
rather than control it. Common settings include:
  - zone_activity_disabled: Exclude sensor from zone activity detection
  - climate_exclude: Exclude device from climate control

Examples:
  homeyctl devices set-setting "Motion Sensor" zone_activity_disabled true
  homeyctl devices set-setting "Thermostat" climate_exclude false`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		settingKey := args[1]
		valueStr := args[2]

		// Find device ID
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		var deviceID, deviceName string
		for _, d := range devices {
			if d.ID == nameOrID || strings.EqualFold(d.Name, nameOrID) {
				deviceID = d.ID
				deviceName = d.Name
				break
			}
		}

		if deviceID == "" {
			return fmt.Errorf("device not found: %s", nameOrID)
		}

		// Parse value
		var value interface{}
		if valueStr == "true" {
			value = true
		} else if valueStr == "false" {
			value = false
		} else {
			// Try as number
			var num float64
			if _, err := fmt.Sscanf(valueStr, "%f", &num); err == nil {
				value = num
			} else {
				value = valueStr
			}
		}

		settings := map[string]interface{}{
			settingKey: value,
		}

		if err := apiClient.SetDeviceSetting(deviceID, settings); err != nil {
			if strings.Contains(err.Error(), "Missing Scopes") {
				return fmt.Errorf(`permission denied: changing device settings requires 'homey.device' scope

OAuth tokens only support 'homey.device.control' (for on/off, dim, etc.),
not full device access needed for settings.

To change device settings, create an API key at my.homey.app:
  1. Go to https://my.homey.app
  2. Select your Homey → Settings → API Keys
  3. Create a new API key (it will have full access)
  4. Run: homeyctl config set-token <your-api-key>`)
			}
			return err
		}

		fmt.Printf("Set %s setting %s = %v\n", deviceName, settingKey, value)
		return nil
	},
}

var devicesGetSettingsCmd = &cobra.Command{
	Use:   "get-settings <name-or-id>",
	Short: "Get device settings",
	Long: `Get all settings for a device.

This shows configurable settings like zone_activity_disabled, climate_exclude,
and driver-specific settings.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		// Find device ID
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		var deviceID, deviceName string
		for _, d := range devices {
			if d.ID == nameOrID || strings.EqualFold(d.Name, nameOrID) {
				deviceID = d.ID
				deviceName = d.Name
				break
			}
		}

		if deviceID == "" {
			return fmt.Errorf("device not found: %s", nameOrID)
		}

		settings, err := apiClient.GetDeviceSettings(deviceID)
		if err != nil {
			return err
		}

		if isTableFormat() {
			var settingsMap map[string]interface{}
			if err := json.Unmarshal(settings, &settingsMap); err != nil {
				return fmt.Errorf("failed to parse settings: %w", err)
			}

			fmt.Printf("Settings for %s:\n\n", deviceName)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SETTING\tVALUE")
			fmt.Fprintln(w, "-------\t-----")
			for key, val := range settingsMap {
				fmt.Fprintf(w, "%s\t%v\n", key, val)
			}
			w.Flush()
			return nil
		}

		outputJSON(settings)
		return nil
	},
}

var devicesDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a device",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		// Get all devices to find by name
		data, err := apiClient.GetDevices()
		if err != nil {
			return err
		}

		var devices map[string]Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return fmt.Errorf("failed to parse devices: %w", err)
		}

		// Find device by name or ID
		for _, d := range devices {
			if d.ID == nameOrID || strings.EqualFold(d.Name, nameOrID) {
				if err := apiClient.DeleteDevice(d.ID); err != nil {
					return err
				}
				fmt.Printf("Deleted device: %s\n", d.Name)
				return nil
			}
		}

		return fmt.Errorf("device not found: %s", nameOrID)
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.AddCommand(devicesListCmd)
	devicesCmd.AddCommand(devicesGetCmd)
	devicesCmd.AddCommand(devicesSetCmd)
	devicesCmd.AddCommand(devicesSetSettingCmd)
	devicesCmd.AddCommand(devicesGetSettingsCmd)
	devicesCmd.AddCommand(devicesDeleteCmd)
}

package cmd

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fishfisher/homeyctl/internal/logic"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// parseValue converts a string value to the appropriate type (bool, number, or string)
func parseValue(valueStr string) interface{} {
	if valueStr == "true" {
		return true
	}
	if valueStr == "false" {
		return false
	}

	// Try as number
	if num, err := strconv.ParseFloat(valueStr, 64); err == nil && !math.IsNaN(num) && !math.IsInf(num, 0) {
		return num
	}

	return valueStr
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

		device, err := findDevice(nameOrID)
		if err != nil {
			return err
		}

		value := parseValue(valueStr)
		definition, exists := device.CapabilitiesObj[capability]
		if !exists {
			return fmt.Errorf("device %q has no capability %q", device.Name, capability)
		}
		if definition.Type != "" && definition.Type != "enum" {
			value, err = logic.ParseValue(definition.Type, valueStr)
			if err != nil {
				return err
			}
		} else if definition.Type == "enum" {
			value = valueStr
		}
		if err := validateCapabilityValue(definition, value); err != nil {
			return fmt.Errorf("%s.%s: %w", device.Name, capability, err)
		}

		if err := apiClient.SetCapability(device.ID, capability, value); err != nil {
			return err
		}

		color.Green("Set %s.%s = %v\n", device.Name, capability, value)
		return nil
	},
}

var devicesOnCmd = &cobra.Command{
	Use:   "on <name-or-id>",
	Short: "Turn device on",
	Long: `Turn a device on (shorthand for 'devices set <name> onoff true').

Examples:
  homeyctl devices on "Living Room Light"
  homeyctl devices on "Aksels rom"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setDeviceOnOff(args[0], true)
	},
}

var devicesOffCmd = &cobra.Command{
	Use:   "off <name-or-id>",
	Short: "Turn device off",
	Long: `Turn a device off (shorthand for 'devices set <name> onoff false').

Examples:
  homeyctl devices off "Living Room Light"
  homeyctl devices off "Aksels rom"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setDeviceOnOff(args[0], false)
	},
}

func setDeviceOnOff(nameOrID string, on bool) error {
	device, err := findDevice(nameOrID)
	if err != nil {
		return err
	}

	// Check if device supports onoff
	if _, hasOnOff := device.CapabilitiesObj["onoff"]; !hasOnOff {
		return fmt.Errorf("device '%s' does not support on/off", device.Name)
	}
	if err := validateCapabilityValue(device.CapabilitiesObj["onoff"], on); err != nil {
		return err
	}

	if err := apiClient.SetCapability(device.ID, "onoff", on); err != nil {
		return err
	}

	state := "on"
	if !on {
		state = "off"
	}
	color.Green("Turned %s %s\n", device.Name, state)
	return nil
}

func validateCapabilityValue(capability Capability, value any) error {
	if capability.Setable != nil && !*capability.Setable {
		return fmt.Errorf("capability is read-only")
	}
	if number, ok := value.(float64); ok {
		if capability.Min != nil && number < *capability.Min {
			return fmt.Errorf("value is below minimum %g", *capability.Min)
		}
		if capability.Max != nil && number > *capability.Max {
			return fmt.Errorf("value is above maximum %g", *capability.Max)
		}
	}
	if capability.Type == "enum" && len(capability.Values) > 0 {
		for _, option := range capability.Values {
			if value == option.ID {
				return nil
			}
		}
		return fmt.Errorf("value is not an allowed enum option")
	}
	return nil
}

func init() {
	devicesCmd.AddCommand(devicesSetCmd)
	devicesCmd.AddCommand(devicesOnCmd)
	devicesCmd.AddCommand(devicesOffCmd)
}

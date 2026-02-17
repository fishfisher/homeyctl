package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

var weatherCmd = &cobra.Command{
	Use:   "weather",
	Short: "Get weather information",
	Long:  `Get current weather and forecasts from Homey.`,
}

var weatherCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Get current weather",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetWeather()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var weather struct {
			State       string  `json:"state"`
			Temperature float64 `json:"temperature"`
			Humidity    float64 `json:"humidity"`
			Pressure    float64 `json:"pressure"`
		}
		if err := json.Unmarshal(data, &weather); err != nil {
			return err
		}

		color.New(color.Bold).Println("Current Weather")
		fmt.Printf("  Condition:   %s\n", weather.State)
		fmt.Printf("  Temperature: %.1f°C\n", weather.Temperature)
		fmt.Printf("  Humidity:    %.0f%%\n", weather.Humidity)
		fmt.Printf("  Pressure:    %.0f hPa\n", weather.Pressure)
		return nil
	},
}

var weatherForecastCmd = &cobra.Command{
	Use:   "forecast",
	Short: "Get hourly weather forecast",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.GetWeatherForecast()
		if err != nil {
			return err
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var forecast []struct {
			Time        string  `json:"time"`
			State       string  `json:"state"`
			Temperature float64 `json:"temperature"`
			Humidity    float64 `json:"humidity"`
		}
		if err := json.Unmarshal(data, &forecast); err != nil {
			return err
		}

		if len(forecast) == 0 {
			fmt.Println("No forecast data available.")
			return nil
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("Time", "Condition", "Temp", "Humidity")
		tbl.WithHeaderFormatter(headerFmt)
		for _, f := range forecast {
			tbl.AddRow(f.Time, f.State, fmt.Sprintf("%.1f°C", f.Temperature), fmt.Sprintf("%.0f%%", f.Humidity))
		}
		tbl.Print()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(weatherCmd)

	weatherCmd.AddCommand(weatherCurrentCmd)
	weatherCmd.AddCommand(weatherForecastCmd)
}

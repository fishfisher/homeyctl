package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fishfisher/homeyctl/internal/client"
	"github.com/fishfisher/homeyctl/internal/config"
	"github.com/fishfisher/homeyctl/internal/oauth"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

// PAT represents a Personal Access Token from the API
type PAT struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"createdAt"`
}

// PATCreateResponse is the response from creating a PAT
type PATCreateResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	Token     string   `json:"token"`
	CreatedAt string   `json:"createdAt"`
}

// Available scopes in Homey (from constants.mts)
var availableScopes = []string{
	"homey",                    // Full access (everything)
	"homey.alarm",              // Alarm full access
	"homey.alarm.readonly",     // Alarm read-only
	"homey.app",                // App full access
	"homey.app.readonly",       // App read-only
	"homey.app.control",        // App control
	"homey.dashboard",          // Dashboard full access
	"homey.dashboard.readonly", // Dashboard read-only
	"homey.energy",             // Energy full access
	"homey.energy.readonly",    // Energy read-only
	"homey.system",             // System full access
	"homey.system.readonly",    // System read-only
	"homey.user",               // User full access
	"homey.user.readonly",      // User read-only
	"homey.user.self",          // User self management
	"homey.updates",            // Updates full access
	"homey.updates.readonly",   // Updates read-only
	"homey.geolocation",        // Geolocation full access
	"homey.geolocation.readonly",
	"homey.device",          // Device full access
	"homey.device.readonly", // Device read-only
	"homey.device.control",  // Device control
	"homey.flow",            // Flow full access
	"homey.flow.readonly",   // Flow read-only
	"homey.flow.start",      // Flow trigger/start
	"homey.insights",        // Insights full access
	"homey.insights.readonly",
	"homey.logic", // Logic/variables full access
	"homey.logic.readonly",
	"homey.mood", // Mood full access
	"homey.mood.readonly",
	"homey.mood.set",
	"homey.notifications", // Notifications full access
	"homey.notifications.readonly",
	"homey.reminder", // Reminder full access
	"homey.reminder.readonly",
	"homey.presence", // Presence full access
	"homey.presence.readonly",
	"homey.presence.self",
	"homey.speech", // Speech
	"homey.zone",   // Zone full access
	"homey.zone.readonly",
}

// Scope presets for common use cases
// Note: These must match scopes configured on the OAuth app at developer.athom.com
var scopePresets = map[string][]string{
	"readonly": {
		"homey.device.readonly",
		"homey.flow.readonly",
		"homey.zone.readonly",
		"homey.app.readonly",
		"homey.insights.readonly",
		"homey.notifications.readonly",
		"homey.presence.readonly",
	},
	"control": {
		"homey.device.readonly",
		"homey.device.control",
		"homey.flow",
		"homey.zone.readonly",
		"homey.app.readonly",
		"homey.insights.readonly",
		"homey.notifications.readonly",
		"homey.presence.readonly",
	},
	"full": {
		"homey", // Full access to everything
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with your Homey",
	Long: `Authenticate with your Homey smart home hub.

Choose an authentication method:

  1) OAuth Login  - Log in via browser (creates scoped token)
  2) API Key      - Paste a key from my.homey.app Settings > API Keys
  3) Status       - Show current authentication info

Subcommands:
  homeyctl auth login          OAuth browser login
  homeyctl auth api-key <key>  Set API key from my.homey.app
  homeyctl auth status         Show current auth state
  homeyctl auth token          Manage Personal Access Tokens`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("How would you like to authenticate?")
		fmt.Println()
		fmt.Println("  1) OAuth Login  — Log in via browser (creates scoped token)")
		fmt.Println("  2) API Key      — Paste a key from my.homey.app > Settings > API Keys")
		fmt.Println("  3) Status       — Show current authentication info")
		fmt.Println()
		fmt.Print("Choose [1-3]: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			return authLoginCmd.RunE(cmd, nil)
		case "2":
			fmt.Print("Paste your API key: ")
			key, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("API key cannot be empty")
			}
			return runAPIKey(key)
		case "3":
			return runAuthStatus()
		default:
			return fmt.Errorf("invalid choice: %s (enter 1, 2, or 3)", input)
		}
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via OAuth browser flow",
	Long: `Log in to your Homey with your Athom account.

This is the easiest way to get started with homeyctl:
  1. Opens your browser to log in with your Athom account
  2. Creates an API token with device control access
  3. Saves it to your config

After login, you can immediately use homeyctl:
  homeyctl devices list
  homeyctl flows list

Note: OAuth tokens are scoped. For full access (including flow updates),
use an API key instead: homeyctl auth api-key <key>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Logging in to your Homey...")
		fmt.Println()

		// Do OAuth login
		homey, err := oauth.Login()
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		// Determine which URL to use
		homeyURL := homey.LocalURLSecure
		if homeyURL == "" {
			homeyURL = homey.LocalURL
		}
		if homeyURL == "" {
			homeyURL = homey.RemoteURL
		}

		// Parse URL
		parsedURL, err := url.Parse(homeyURL)
		if err != nil {
			return fmt.Errorf("failed to parse Homey URL: %w", err)
		}

		host := parsedURL.Hostname()
		port := 443
		if parsedURL.Port() != "" {
			fmt.Sscanf(parsedURL.Port(), "%d", &port)
		} else if parsedURL.Scheme == "http" {
			port = 80
		}

		// Create temporary config for API client
		tempCfg := &config.Config{
			Host:  host,
			Port:  port,
			Token: homey.Token,
			TLS:   parsedURL.Scheme == "https",
		}

		// Create a "control" preset token for the user
		tempClient := client.New(tempCfg)
		scopes := scopePresets["control"]

		data, err := tempClient.CreatePAT("homeyctl", scopes)
		if err != nil {
			// If token creation fails, save the OAuth session token instead
			// (less ideal but still works)
			fmt.Println("Note: Could not create scoped token, using session token.")
			if saveErr := config.Save(tempCfg); saveErr != nil {
				return fmt.Errorf("failed to save config: %w", saveErr)
			}
			fmt.Println()
			fmt.Printf("Logged in to: %s\n", homey.Name)
			fmt.Println()
			fmt.Println("You're ready to use homeyctl!")
			fmt.Println("Try: homeyctl devices list")
			return nil
		}

		var resp PATCreateResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Save the PAT to config
		newCfg := &config.Config{
			Host:  host,
			Port:  port,
			Token: resp.Token,
			TLS:   parsedURL.Scheme == "https",
		}

		if err := config.Save(newCfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println()
		fmt.Printf("Logged in to: %s\n", homey.Name)
		fmt.Println()
		fmt.Println("You're ready to use homeyctl!")
		fmt.Println("Try: homeyctl devices list")

		return nil
	},
}

var authAPIKeyCmd = &cobra.Command{
	Use:   "api-key <token>",
	Short: "Set API key from my.homey.app",
	Long: `Set an API key created from the Homey web interface.

How to create an API key:
  1. Go to https://my.homey.app/
  2. Select your Homey
  3. Click Settings (gear icon, bottom left)
  4. Click API Keys
  5. Click "+ New API Key"
  6. Copy the generated key and paste it here

API keys give full access to your Homey (no scope limitations unlike
OAuth-created tokens). This is the best method for full control of
flows, devices, and all other Homey features.

Example:
  homeyctl auth api-key abc123def456...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIKey(args[0])
	},
}

func runAPIKey(token string) error {
	loadedCfg, err := config.Load()
	if err != nil {
		loadedCfg = &config.Config{
			Host: "localhost",
			Port: 4859,
		}
	}

	loadedCfg.Token = token

	if err := config.Save(loadedCfg); err != nil {
		return err
	}

	color.Green("API key saved successfully!\n")
	fmt.Println()
	fmt.Println("You're ready to use homeyctl!")
	fmt.Println("Try: homeyctl devices list")
	return nil
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication info",
	Long:  `Display the current authentication method, token, and connection details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthStatus()
	},
}

func runAuthStatus() error {
	loadedCfg, err := config.Load()
	if err != nil {
		fmt.Println("Authentication")
		fmt.Println("==============")
		fmt.Println("Status: Not configured")
		fmt.Println()
		fmt.Println("Run 'homeyctl auth' to get started.")
		return nil
	}

	token := loadedCfg.EffectiveToken()
	if token == "" {
		fmt.Println("Authentication")
		fmt.Println("==============")
		fmt.Println("Status: Not configured")
		fmt.Println()
		fmt.Println("Run 'homeyctl auth' to get started.")
		return nil
	}

	mode := loadedCfg.EffectiveMode()

	fmt.Println("Authentication")
	fmt.Println("==============")
	fmt.Printf("Token:      %s\n", maskToken(token))
	fmt.Printf("Connection: %s", mode)
	if mode == "local" {
		addr := loadedCfg.Local.Address
		if addr == "" && loadedCfg.Host != "localhost" {
			addr = loadedCfg.Host
		}
		if addr != "" {
			fmt.Printf(" (%s)", addr)
		}
	}
	fmt.Println()

	return nil
}

// --- Token management subcommands ---

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage Personal Access Tokens (PAT)",
	Long: `Create and manage scoped Personal Access Tokens for AI bots and integrations.

PATs allow you to create tokens with limited permissions, so you can safely
give access to third-party tools without exposing full control of your Homey.

Note: Creating PATs requires an owner account with OAuth or password login.
PAT tokens cannot be used to create new PATs.`,
}

var authTokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Personal Access Tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apiClient.ListPATs()
		if err != nil {
			if strings.Contains(err.Error(), "Invalid Session Type") {
				return fmt.Errorf("cannot list PATs: you must be logged in with OAuth or password (PAT tokens cannot manage other PATs)")
			}
			return fmt.Errorf("failed to list tokens: %w", err)
		}

		if isJSON() {
			outputJSON(data)
			return nil
		}

		var pats []PAT
		if err := json.Unmarshal(data, &pats); err != nil {
			return fmt.Errorf("failed to parse tokens: %w", err)
		}

		if len(pats) == 0 {
			fmt.Println("No tokens found.")
			return nil
		}

		headerFmt := color.New(color.FgCyan, color.Underline).SprintfFunc()
		tbl := table.New("ID", "Name", "Scopes", "Created")
		tbl.WithHeaderFormatter(headerFmt)
		for _, p := range pats {
			scopes := formatScopes(p.Scopes)
			created := formatTime(p.CreatedAt)
			tbl.AddRow(p.ID, p.Name, scopes, created)
		}
		tbl.Print()
		return nil
	},
}

var authTokenCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new Personal Access Token",
	Long: `Create a new scoped Personal Access Token.

This command will automatically authenticate via OAuth if needed.
By default, the created token is saved to your config for immediate use.

Use --preset for common scope combinations:
  readonly  - Read-only access to devices, flows, zones, etc.
  control   - Read + control devices and full flow access
  full      - Full access (same as owner)

Or use --scopes for specific scopes:
  --scopes homey.device.readonly,homey.flow.readonly

Use --no-save to create a token without saving it (for external use).

Examples:
  homeyctl auth token create "AI Bot" --preset readonly
  homeyctl auth token create "Home Assistant" --preset control
  homeyctl auth token create "External" --preset readonly --no-save`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		preset, _ := cmd.Flags().GetString("preset")
		scopesStr, _ := cmd.Flags().GetString("scopes")
		noSave, _ := cmd.Flags().GetBool("no-save")

		var scopes []string

		if preset != "" && scopesStr != "" {
			return fmt.Errorf("cannot use both --preset and --scopes")
		}

		if preset != "" {
			presetScopes, ok := scopePresets[preset]
			if !ok {
				return fmt.Errorf("unknown preset: %s (available: readonly, control, full)", preset)
			}
			scopes = presetScopes
		} else if scopesStr != "" {
			scopes = strings.Split(scopesStr, ",")
			for i := range scopes {
				scopes[i] = strings.TrimSpace(scopes[i])
			}
		} else {
			return fmt.Errorf("must specify --preset or --scopes")
		}

		// Try to use existing config, or do OAuth login
		existingCfg, _ := config.Load()

		needsOAuth := existingCfg == nil || existingCfg.Token == ""

		if !needsOAuth {
			// Check if current token can manage PATs by listing them
			tempClient := client.New(existingCfg)
			_, err := tempClient.ListPATs()
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Invalid Session Type") {
					// Current token is a PAT, need OAuth
					needsOAuth = true
				}
				// Other errors might just be network issues, we'll catch them later
			}
		}

		// Need OAuth login
		if needsOAuth {
			fmt.Println("OAuth authentication required to create tokens...")
			homey, err := oauth.Login()
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			// Determine which URL to use
			homeyURL := homey.LocalURLSecure
			if homeyURL == "" {
				homeyURL = homey.LocalURL
			}
			if homeyURL == "" {
				homeyURL = homey.RemoteURL
			}

			// Create a temporary client with the OAuth session
			parsedURL, err := url.Parse(homeyURL)
			if err != nil {
				return fmt.Errorf("failed to parse Homey URL: %w", err)
			}

			host := parsedURL.Hostname()
			port := 443
			if parsedURL.Port() != "" {
				fmt.Sscanf(parsedURL.Port(), "%d", &port)
			} else if parsedURL.Scheme == "http" {
				port = 80
			}

			existingCfg = &config.Config{
				Host:  host,
				Port:  port,
				Token: homey.Token,
				TLS:   parsedURL.Scheme == "https",
			}
		}

		// Create the PAT
		tempClient := client.New(existingCfg)
		data, err := tempClient.CreatePAT(name, scopes)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "Invalid Session Type") {
				return fmt.Errorf("cannot create PAT: authentication failed. Please try again")
			}
			if strings.Contains(errStr, "Must Be Owner") {
				return fmt.Errorf("cannot create PAT: only the owner account can create tokens")
			}
			if strings.Contains(errStr, "Missing Scopes") {
				return fmt.Errorf("cannot create PAT: the requested scopes are not available.\nTry a different preset or check available scopes with: homeyctl auth token scopes")
			}
			return fmt.Errorf("failed to create token: %w", err)
		}

		var resp PATCreateResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		fmt.Println()
		color.Green("Token created successfully!\n")
		fmt.Printf("Name:   %s\n", resp.Name)
		fmt.Printf("Scopes: %s\n", strings.Join(resp.Scopes, ", "))

		if noSave {
			// Just print the token
			fmt.Println()
			fmt.Printf("Token: %s\n", resp.Token)
			fmt.Println()
			fmt.Println("IMPORTANT: Save this token now - it cannot be retrieved later.")
		} else {
			// Save the PAT to config
			newCfg := &config.Config{
				Host:  existingCfg.Host,
				Port:  existingCfg.Port,
				Token: resp.Token,
				TLS:   existingCfg.TLS,
			}

			if err := config.Save(newCfg); err != nil {
				// Still show the token if save fails
				fmt.Println()
				fmt.Printf("Token: %s\n", resp.Token)
				fmt.Println()
				return fmt.Errorf("token created but failed to save config: %w\nSave it manually with: homeyctl auth api-key <token>", err)
			}

			fmt.Println()
			fmt.Println("Token saved to config. You're ready to use homeyctl!")
			fmt.Printf("Try: homeyctl devices list\n")
		}

		return nil
	},
}

var authTokenDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a Personal Access Token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		if err := apiClient.DeletePAT(id); err != nil {
			return fmt.Errorf("failed to delete token: %w", err)
		}

		color.Green("Token deleted: %s\n", id)
		return nil
	},
}

var authTokenScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "List available scopes",
	Long: `List all available scopes that can be used when creating tokens.

Scopes follow a hierarchy:
  - homey.device includes homey.device.readonly and homey.device.control
  - homey.flow includes homey.flow.readonly and homey.flow.start
  - homey (full access) includes all scopes`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available scopes:")
		fmt.Println()
		fmt.Println("PRESETS:")
		fmt.Println("  readonly  - Read-only access to all resources")
		fmt.Println("  control   - Read + control devices, full flow access")
		fmt.Println("  full      - Full access (same as owner)")
		fmt.Println()
		fmt.Println("INDIVIDUAL SCOPES:")
		for _, scope := range availableScopes {
			fmt.Printf("  %s\n", scope)
		}
	},
}

func formatScopes(scopes []string) string {
	if len(scopes) == 0 {
		return "-"
	}
	if len(scopes) == 1 {
		return scopes[0]
	}
	if len(scopes) <= 3 {
		return strings.Join(scopes, ", ")
	}
	return fmt.Sprintf("%s, +%d more", scopes[0], len(scopes)-1)
}

func formatTime(timeStr string) string {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr
	}
	return t.Format("2006-01-02")
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authAPIKeyCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authTokenCmd)
	authTokenCmd.AddCommand(authTokenListCmd)
	authTokenCmd.AddCommand(authTokenCreateCmd)
	authTokenCmd.AddCommand(authTokenDeleteCmd)
	authTokenCmd.AddCommand(authTokenScopesCmd)

	authTokenCreateCmd.Flags().String("preset", "", "Scope preset: readonly, control, or full")
	authTokenCreateCmd.Flags().String("scopes", "", "Comma-separated list of scopes")
	authTokenCreateCmd.Flags().Bool("no-save", false, "Don't save token to config (for external use)")
}

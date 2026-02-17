---
name: homey
description: Control Homey smart home using the homeyctl CLI. Use when asked to control devices, trigger flows, check energy usage, manage zones/users, moods, presence, weather, or interact with Athom Homey Pro. Covers device control (lights, thermostats, sensors), flow management (list/trigger/create/folders), energy monitoring, insights, variables, system operations, dashboards, moods, presence tracking, and weather.
metadata: {"clawdbot":{"emoji":"🏠","requires":{"bins":["homeyctl"]},"install":[{"id":"brew","kind":"brew","formula":"fishfisher/tap/homeyctl","bins":["homeyctl"],"label":"Install homeyctl (brew)"}]}}
---

# Homey

Control your Athom Homey Pro smart home hub using the `homeyctl` CLI. This skill enables device control, flow automation, energy monitoring, and system management through Homey's local API.

## Setup & Configuration

### Interactive Setup (Easiest)

```bash
homeyctl auth
```

This presents an interactive menu to choose your authentication method.

### OAuth Login (Browser)

```bash
homeyctl auth login
```

Opens your browser to log in with your Athom account. Creates a scoped token automatically.

**Note:** OAuth tokens have scope limitations. For full access (including flow updates), use an API key instead.

### API Key (Recommended for Power Users)

```bash
homeyctl auth api-key <token>
```

API keys from my.homey.app give full access with no scope limitations:
1. Go to https://my.homey.app/ → Settings → API Keys → New API Key
2. Copy the key and run: `homeyctl auth api-key <key>`

### Check Auth Status

```bash
homeyctl auth status
```

### Verify Setup

```bash
homeyctl devices list
```

Configuration is stored in `~/Library/Application Support/homeyctl/config.toml` (macOS).

## Quick Start

```bash
# Get complete system snapshot
homeyctl snapshot
homeyctl snapshot --include-flows

# List all devices
homeyctl devices list

# Control a device (multiple ways)
homeyctl devices on "Living Room Light"      # Shortcut
homeyctl devices off "Living Room Light"     # Shortcut
homeyctl devices set "Living Room Light" dim 0.5

# Trigger a flow
homeyctl flows trigger "Good Morning"

# Activate a mood
homeyctl moods set "Movie Time"

# Check presence and weather
homeyctl presence get me
homeyctl weather current

# Check energy usage
homeyctl energy live
homeyctl energy report day
```

## Security: Token Scoping

**Important:** For AI assistants, use scoped tokens to prevent accidental changes.

The user should create a readonly token for you:
```bash
homeyctl auth token create "AI Bot" --preset readonly --no-save
```

Available presets:
- **readonly** - Safe for AI exploration (list/get only)
- **control** - Read + control devices, full flow access
- **full** - Full access (same as owner)

If you try an operation without permissions, you'll see: `Error: 403 Missing Scopes`

## Core Capabilities

### 1. Device Control

List and control all connected smart home devices:

```bash
# List all devices (lights, sensors, thermostats, etc.)
homeyctl devices list
homeyctl devices list --filter "living"  # Filter by name pattern

# Get device details (capabilities, state, zone)
homeyctl devices get "Living Room Light"

# Control devices (multiple ways)
homeyctl devices on "Living Room Light"           # Shortcut (turns on)
homeyctl devices off "Living Room Light"          # Shortcut (turns off)
homeyctl devices set "Living Room Light" dim 0.5  # Set specific capability
homeyctl devices set "Thermostat" target_temperature 22

# Device management
homeyctl devices rename "Old Name" "New Name"
homeyctl devices move "Device" "Kitchen"          # Move to zone
homeyctl devices hide "Device"                    # Hide from views
homeyctl devices delete "Old Device"

# Device settings (separate from capabilities)
homeyctl devices settings get "Device"
homeyctl devices settings set "Device" setting_name value
```

### 2. Flow Management

Manage Homey flows (automations):

```bash
# List all flows
homeyctl flows list
homeyctl flows list --match "morning"  # Filter by name pattern

# Get flow details (use to see structure)
homeyctl flows get "My Flow"

# Trigger a flow by name
homeyctl flows trigger "Good Morning"

# Update existing flow (merge) — file, inline JSON, or stdin
homeyctl flows update "My Flow" updated-flow.json
homeyctl flows update "My Flow" --data '{"name": "New Name"}'
echo '{"name": "New"}' | homeyctl flows update "My Flow" -

# Delete a flow
homeyctl flows delete "Old Flow"

# List available flow cards (for creating flows)
homeyctl flows cards --type trigger
homeyctl flows cards --type condition
homeyctl flows cards --type action

# Flow folders (organize flows)
homeyctl flows folders list
homeyctl flows folders create "Automation"
homeyctl flows folders get "Automation"
homeyctl flows folders update "Automation" updated-folder.json
homeyctl flows folders delete "Old Folder"
```

#### Flow JSON Format

```json
{
  "name": "Turn on lights when arriving",
  "trigger": {
    "id": "homey:manager:presence:user_enter",
    "args": { "user": "user-uuid-here" }
  },
  "conditions": [
    {
      "id": "homey:manager:logic:lt",
      "args": { "value": 20 },
      "droptoken": "homey:device:<device-id>|measure_temperature"
    }
  ],
  "actions": [
    {
      "id": "homey:device:<device-id>:thermostat_mode_heat",
      "args": { "mode": "heat" }
    }
  ]
}
```

**Critical: Droptoken format uses pipe (`|`), not colon:**
```
CORRECT: "homey:device:abc123|measure_temperature"
WRONG:   "homey:device:abc123:measure_temperature"
```

**ID Format Reference:**

| Type | Format | Example |
|------|--------|---------|
| Device action | homey:device:\<id\>:\<capability\> | homey:device:abc123:on |
| Manager trigger | homey:manager:\<manager\>:\<event\> | homey:manager:presence:user_enter |
| Logic condition | homey:manager:logic:\<operator\> | homey:manager:logic:lt |
| Droptoken | homey:device:\<id\>\|\<capability\> | homey:device:abc123\|measure_temperature |

**Common Triggers:**
- `homey:manager:presence:user_enter` - User arrives home
- `homey:manager:presence:user_leave` - User leaves home
- `homey:manager:time:time` - At specific time
- `homey:device:<id>:<capability>_changed` - Device state changes

**Common Conditions:**
- `homey:manager:logic:lt` - Less than (use with droptoken)
- `homey:manager:logic:gt` - Greater than (use with droptoken)
- `homey:manager:logic:eq` - Equals (use with droptoken)

**Flow Update Behavior:**

`homeyctl flows update` does a **partial/merge update**:
- Only fields you include will be changed
- Omitted fields keep their existing values
- To remove conditions/actions, explicitly set empty array: `"conditions": []`

```bash
# Rename a flow (inline JSON — no temp file needed)
homeyctl flows update "Old Name" --data '{"name": "New Name"}'

# Rename via stdin
echo '{"name": "New Name"}' | homeyctl flows update "Old Name" -

# Rename via file
echo '{"name": "New Name"}' > rename.json
homeyctl flows update "Old Name" rename.json

# Remove all conditions from a flow
homeyctl flows update "My Flow" --data '{"conditions": []}'
```

### 3. Energy Monitoring

Track power usage and electricity prices:

```bash
# Live power usage (W)
homeyctl energy live

# Daily/weekly/monthly/yearly reports
homeyctl energy report day
homeyctl energy report week
homeyctl energy report month --date 2025-12
homeyctl energy report year --date 2025

# Check/set electricity prices
homeyctl energy price                    # Show dynamic prices
homeyctl energy price set 0.50          # Set fixed price (kr/kWh)
homeyctl energy price type              # Show current type
homeyctl energy price type fixed        # Switch to fixed pricing
```

### 4. Insights & Historical Data

Access historical sensor data and logs:

```bash
# List all insight logs
homeyctl insights list

# Get historical data for a device
homeyctl insights get "homey:device:abc123:measure_power"

# Different time resolutions
homeyctl insights get "homey:device:abc123:measure_power" --resolution lastWeek

# Delete insight log
homeyctl insights delete "homey:device:abc123:measure_power"
```

### 5. Variables

Manage logic variables used in flows:

```bash
# List all variables
homeyctl variables list

# Get/set variable value
homeyctl variables get "my_variable"
homeyctl variables set "my_variable" 42

# Create/delete variable
homeyctl variables create "new_var" number 0
homeyctl variables delete "new_var"
```

### 6. Zones & Users

Manage rooms and household members:

```bash
# Zones (rooms)
homeyctl zones list
homeyctl zones create "New Room"
homeyctl zones get "Living Room"
homeyctl zones update "Living Room" updated-zone.json
homeyctl zones delete "Unused Room"

# Users (household members)
homeyctl users list
homeyctl users get "User Name"
homeyctl users create "New User" user-data.json
homeyctl users update "User" updated-user.json
homeyctl users delete "Old User"
```

### 7. Apps & System

Manage installed apps and system operations:

```bash
# App management
homeyctl apps list
homeyctl apps get com.some.app
homeyctl apps install com.some.app
homeyctl apps uninstall com.some.app
homeyctl apps enable com.some.app
homeyctl apps disable com.some.app
homeyctl apps restart com.some.app
homeyctl apps settings get com.some.app
homeyctl apps settings set com.some.app setting_name value

# System information and control
homeyctl system info
homeyctl system name                    # Get system name
homeyctl system name set "My Homey"    # Set system name
homeyctl system reboot                 # Reboot Homey (requires confirmation)
```

### 8. Notifications

Send notifications to Homey timeline:

```bash
# Send notification
homeyctl notifications send "Hello from CLI"

# List recent notifications
homeyctl notifications list

# Delete notification
homeyctl notifications delete <notification-id>
```

### 9. Moods

Manage and activate room moods (lighting scenes):

```bash
# List all moods
homeyctl moods list

# Get mood details
homeyctl moods get "Movie Time"

# Activate a mood
homeyctl moods set "Movie Time"

# Create new mood
homeyctl moods create mood-data.json

# Update mood
homeyctl moods update "Movie Time" updated-mood.json

# Delete mood
homeyctl moods delete "Old Mood"
```

### 10. Presence Tracking

Track user presence (home/away) and sleep status:

```bash
# Get user presence
homeyctl presence get me
homeyctl presence get "User Name"

# Set presence (home/away)
homeyctl presence set me home
homeyctl presence set me away
homeyctl presence set "User Name" home

# Get sleep status
homeyctl presence asleep get me
homeyctl presence asleep get "User Name"

# Set sleep status
homeyctl presence asleep set me true
homeyctl presence asleep set me false
```

### 11. Weather

Get current weather and forecasts from Homey:

```bash
# Current weather conditions
homeyctl weather current

# Hourly forecast
homeyctl weather forecast
```

### 12. Dashboards

Manage Homey dashboards:

```bash
# List all dashboards
homeyctl dashboards list

# Get dashboard details
homeyctl dashboards get "Main Dashboard"

# Create dashboard
homeyctl dashboards create dashboard-data.json

# Update dashboard
homeyctl dashboards update "Dashboard" updated-dashboard.json

# Delete dashboard
homeyctl dashboards delete "Old Dashboard"
```

### 13. Snapshot

Get complete Homey system state in one call (useful for AI context):

```bash
# Get system snapshot (devices, zones, users, etc.)
homeyctl snapshot

# Include flows in snapshot
homeyctl snapshot --include-flows

# JSON output for scripting
homeyctl snapshot --json
```

### 14. HomeyScript

Manage and run HomeyScript scripts, and create flows that run them:

```bash
# List all scripts
homeyctl hs list

# Get script details (includes source code)
homeyctl hs get "my_script"

# Create a script
homeyctl hs create "my_script" --file script.js
homeyctl hs create "my_script" --code 'console.log("Hello")'

# Update a script
homeyctl hs update "my_script" --file updated.js
homeyctl hs update "my_script" --name "new_name"

# Run a script (with optional arguments)
homeyctl hs run "my_script"
homeyctl hs run "my_script" arg1 arg2

# Delete a script
homeyctl hs delete "my_script"

# Create a triggerable flow that runs a HomeyScript
homeyctl hs create-flow "my_script" --name "Run My Script"
homeyctl hs create-flow "my_script" --name "Run With Arg" --arg "some|argument"
```

The `create-flow` command builds a simple flow with the "This flow is started" trigger
wired to a HomeyScript action card. Trigger it with `homeyctl flows trigger "Run My Script"`.

## Output Formats

```bash
# Human-readable output (default)
homeyctl devices list

# JSON output (machine-readable)
homeyctl devices list --json
```

All list commands with `--json` return flat JSON arrays for easy parsing:
```bash
# Find flow by name
homeyctl flows list --json | jq '.[] | select(.name | test("pult";"i"))'

# Get all enabled flows
homeyctl flows list --json | jq '.[] | select(.enabled)'

# Get device IDs by name
homeyctl devices list --json | jq '.[] | select(.name | test("office";"i")) | .id'
```

## Connection Modes

```bash
homeyctl config set-mode auto     # Auto-detect best connection (default)
homeyctl config set-mode local    # Force local network connection
homeyctl config set-mode cloud    # Force cloud connection
```

## Workflow Tips

1. **Use snapshot for context**: Run `homeyctl snapshot` to get complete system overview in one call
2. **Filter lists**: Use `--filter "name"` on list commands to quickly find items
3. **Device shortcuts**: Use `homeyctl devices on/off <device>` instead of setting `onoff` capability
4. **Get device IDs first**: Run `homeyctl devices list` to find device IDs
5. **Get user IDs**: Run `homeyctl users list` for presence triggers
6. **Check capabilities**: Run `homeyctl devices get <id>` to see available capabilities
7. **Validate before creating**: The CLI validates flow JSON and warns about common mistakes
8. **Test flows**: Use `homeyctl flows trigger "Flow Name"` to test manually

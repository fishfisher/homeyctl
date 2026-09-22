# Command reference

Run `homeyctl <group> <command> --help` for the installed build's exact flags. Use `--json` for machine-readable discovery. `homeyctl` is distinct from Athom's `homey` CLI.

## Connection

```bash
homeyctl auth
homeyctl auth login
homeyctl auth status
homeyctl config show --json
homeyctl config discover
```

Configuration lives in the OS user-config directory. On macOS: `~/Library/Application Support/homeyctl/config.toml`. Existing keys should remain private. Use environment variables for automation where appropriate: `HOMEY_MODE`, `HOMEY_LOCAL_ADDRESS`, `HOMEY_LOCAL_TOKEN`, `HOMEY_CLOUD_ADDRESS`, `HOMEY_CLOUD_TOKEN`. Cloud mode requires an explicit HTTPS API address for the selected Homey. A remote Homey Pro connect URL can be used with a suitable Homey API key. Auto mode selects configured local settings first; it does not provide network failover. Do not guess or change the selected Homey.

## Devices and discovery

```bash
homeyctl snapshot --include-flows --json
homeyctl devices list --match kitchen --json
homeyctl devices list --zone "Living room" --json   # Includes zones beneath it
homeyctl devices get <id> --json
homeyctl devices values <id> --json
homeyctl zones list --json
homeyctl users list --json
```

For authorized control, use `devices on <id>`, `devices off <id>`, or `devices set <id> <capability> <value>`. Read capabilities first: not every device supports the same controls. Device settings use `devices get-settings <id>` and `devices set-setting <id> <setting> <value>`.

## Builder

```bash
homeyctl flows list --json
homeyctl flows list --folder "AI Flows" --disabled --json
homeyctl flows get <id> --json
homeyctl flows audit --json
homeyctl flows audit --problems --json
homeyctl flows cards --type action --filter logic --json
homeyctl flows autocomplete <card-id> <arg-name> --type action --query text
homeyctl flows validate draft.json --json
homeyctl flows validate draft.json --online --json
homeyctl flows create draft.json --ai --dry-run
homeyctl flows create draft.json --ai --json
homeyctl flows update <id> patch.json --dry-run
homeyctl flows update <id> patch.json --json
homeyctl flows enable <id> --json        # Preview effects; changes nothing
homeyctl flows enable <id> --yes --json  # Only after the user authorized the effects
homeyctl flows disable <id> --json
homeyctl flows restore <backup-file> --dry-run
homeyctl flows restore <backup-file> --json
homeyctl flows folders list --json
homeyctl variables list --json
homeyctl variables batch variables.json --json
```

`flows validate` works offline by default. Creation detects Advanced Flows from `cards`; `--advanced` can also be supplied. Ordinary flows get missing empty actions/conditions arrays and group defaults. `flows update` always backs up the existing document, validates the merged result, and reads it back. `--backup` is a deprecated no-op kept for old scripts. `--replace-cards` removes cards omitted from a complete advanced graph.

`flows restore <backup-file>` reapplies a backup as the complete document for the flow named by the backup's own id, or by `--to`. It backs up the current state first, validates, and reads the result back. Cards, conditions, and actions added since the backup are removed, so preview with `--dry-run` first. A deleted flow cannot be restored in place because its id is gone.

`flows delete <id> --force` backs up before deletion. `variables delete <id> --force` and `variables set <id> <value>` also save the previous variable. These commands require user authorization for their effects. `flows trigger <id>` executes a flow; it is not a validator.

## Other capabilities

Use `moods list` and `moods set <name-or-id>` for moods. Use `presence get <user>` and consult `presence --help` before changes. Use `weather current`, `energy live`, `energy report day`, and `insights --help` for readings. Use `apps list` for installed integrations.

Inspect `homeyscript --help` before script operations. Read a script before execution and treat writes inside it like direct device or flow changes. Prefer native cards for ordinary automation; use HomeyScript for an explicitly justified gap. Arbitrary scripts can bypass CLI validation and backup protections, so do not use them as an escape hatch.

Use `system --help` for system operations. Reboot, app removal, device deletion, and other disruptive actions are separate from a flow-building request and require specific authorization.

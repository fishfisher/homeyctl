# Repository Guidelines

## Project Structure
- `cmd/` - Cobra command definitions. Each file is a command group (devices, flows, zones, etc.)
- `internal/client/` - HTTP client for Homey's local REST API
- `internal/config/` - Configuration management with Viper
- `main.go` - Entry point with version info

## Build & Test Commands
- `go build -o homeyctl .` - Build the CLI binary
- `go test ./...` - Run the full test suite
- `make test` - Same as above
- `make fmt` - Format with gofumpt
- `make lint` - Run golangci-lint

## Coding Style
- Go formatting via `gofumpt` (run `make fmt`)
- Lint via `golangci-lint` (run `make lint`)
- Follow standard Go naming: exported `CamelCase`, unexported `camelCase`

## Testing Guidelines
- Use Go's `testing` package
- Run `go test ./...` before shipping changes
- Tests live alongside code as `*_test.go`

## Commit Guidelines
- Conventional Commit style: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- Keep commits scoped and descriptive
- Include `Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>` when AI-assisted

## Configuration
- Config file: the OS config directory (`~/Library/Application Support/homeyctl/config.toml` on macOS)
- Environment variables: `HOMEY_HOST`, `HOMEY_TOKEN`, `HOMEY_LOCAL_ADDRESS`, `HOMEY_LOCAL_TOKEN`, `HOMEY_CLOUD_TOKEN`, `HOMEY_FORMAT`

## Flow Creation (AI Agents)
When creating flows via CLI:

Read `homey-skill/SKILL.md` and its relevant references. Create new AI flows with
`--ai` so they remain disabled in the AI Flows review folder. Validate locally
and online before creation. Use `--dry-run` to preview changes. Existing flows
must be backed up before every update/delete; the CLI enforces this.

Use `variables batch` to plan persistent state. Follow the skill's approval
thresholds for 5+, 10+, and 30+ new variables, counting the entire operation.
Never bypass review by splitting batches or using raw API/HomeyScript writes.
Do not trigger or enable drafts until the user authorizes their effects.
`flows enable <id>` without `--yes` shows those effects and changes nothing;
add `--yes` only after the user has approved them.

1. **Discover IDs first:**
   ```bash
   homeyctl devices list          # Get device IDs
   homeyctl users list            # Get user IDs
   homeyctl flows cards --type trigger|condition|action  # Get card IDs
   homeyctl zones list            # Get zone IDs
   ```

2. **Droptoken format for logic conditions:**
   - Use pipe (`|`) before capability: `homey:device:<device-id>|<capability>`
   - Example: `homey:device:abc123|measure_temperature`

3. **Flow JSON structure:**
   ```json
   {
     "name": "Flow Name",
     "trigger": {"id": "homey:manager:presence:user_enter", "args": {...}},
     "conditions": [{"id": "...", "droptoken": "...", "args": {...}}],
     "actions": [{"id": "homey:device:<id>:on", "args": {}}]
   }
   ```

4. **Validation:** CLI validates JSON before sending to API

## Common Tasks
- List devices: `homeyctl devices list`
- Control device: `homeyctl devices set "Device Name" capability value`
- Trigger flow: `homeyctl flows trigger "Flow Name"`
- Create flow: `homeyctl flows create flow.json`

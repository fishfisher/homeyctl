# Advanced Flow construction

Discover the target Homey's card definitions and inspect a related stored flow before writing JSON. The top-level writable fields are `name`, `folder`, `enabled`, and `cards`. Treat the response's `id`, `broken`, and other server metadata as read-only.

`cards` is an object keyed by canvas-card identifier. Use newly generated UUIDs for new cards. Existing flows may contain custom keys: preserve them when editing, including all references. The canvas key is different from an installed card's `id` such as `homey:device:<device-id>:<action>`.

## Eight card types

| Type | Content | Usual outgoing fields |
| --- | --- | --- |
| `trigger` | Installed `id`, discovered `args`, optional `ownerUri` | `outputSuccess` |
| `condition` | Installed `id`, discovered `args`, optional `inverted` and `droptoken` | `outputTrue`, `outputFalse`, `outputError` |
| `action` | Installed `id`, discovered `args`, optional `droptoken` | `outputSuccess`, `outputError` |
| `start` | Manual entry point | `outputSuccess` |
| `delay` | `args.delay` | `outputSuccess` |
| `all` | Explicit `input` array joining upstream ports | `outputSuccess` |
| `any` | Join allowing an incoming branch to continue | `outputSuccess` |
| `note` | `value`, `color`, optional `width` and `height` | None |

Include numeric `x` and `y` coordinates on every card. Note colors observed in Homey are `yellow`, `red`, `green`, and `blue`. Preserve exported fields that the card uses, even if not shown in this summary.

Outputs are arrays of canvas-card keys, for example `"outputTrue": ["<next-card-key>"]`. Keep a condition's true and false branches distinct. Route error outputs deliberately where useful; an error path should not accidentally report successful execution. Every referenced target must exist. Do not connect execution edges to notes or trigger/start entry cards.

An ALL input names both the source card and its output port:

```json
{
  "type": "all",
  "x": 800,
  "y": 200,
  "input": ["<condition-key>::outputTrue", "<action-key>::outputSuccess"],
  "outputSuccess": ["<next-card-key>"]
}
```

Both named source ports must also point to the ALL card. Use ALL only where both branches can actually complete in the same intended execution. Joining mutually exclusive true/false branches with ALL can wait forever. Use ANY for alternative paths. Real Homey exports commonly omit `input` on ANY; do not require or invent an ALL-style input list merely because an external example includes one.

A delay uses a string number and numeric multiplier:

```json
{"type":"delay","x":400,"y":100,"args":{"delay":{"number":"5","multiplier":60}}}
```

`1` means seconds and `60` means minutes. A delay does not automatically cancel older executions when a new trigger fires. For debounce, occupancy timeouts, and manual overrides, choose an available timer/cancel mechanism or a carefully designed state check and explain its behavior.

## Tokens and arguments

Use the argument definition from the installed card. Autocomplete arguments usually need a selected object containing an ID and label, sometimes additional fields. `args.variable` commonly uses `{"id":"<logic-id>","name":"<name>"}`. Preserve number, boolean, and string JSON types.

Global droptokens use `ownerUri|token`, for example `homey:device:<device-id>|measure_temperature` or `homey:manager:logic|<variable-id>`. Embedded global tags in strings commonly use `[[homey:manager:logic:<variable-id>]]`. Do not interchange embedded-tag and droptoken separators.

Advanced local tokens identify their producing card, for example `trigger::<canvas-key>::<token-name>`; action-produced tokens may use `action::...`. Verify the token exists in the card definition and is available on the branch that consumes it. A token from one trigger is not automatically available on a path entered through another trigger. Ordinary flows may use a bare trigger-token name such as `value`.

An optional `droptoken` may be absent or JSON `null`. Do not turn a missing value into the string `"null"`. Check the requiredness and supported token types of each installed card; syntactically valid tags can still have the wrong type.

## Updating a graph

`flows update` merges top-level fields. Each supplied advanced-card entry replaces the whole card object, not individual nested fields. Fetch first and preserve `type`, `id`, `args`, position, owner metadata, and outputs as appropriate. A card containing only new `args` is incomplete and is rejected.

An omitted card stays unchanged. A card set to `null` is explicitly removed. Remove its incoming edges and ALL input references in the same update. Use `--replace-cards` to treat the provided graph as complete and remove omitted cards; reserve this for an intentional graph replacement or restore.

These are CLI patch semantics. homeyctl builds and sends the complete resulting graph, without null card entries, and checks the stored graph afterward.

Use `flows update <id> file.json --dry-run` to inspect the merged payload before applying. Every write requires a successful backup and is followed by a read-back check. On a read-back mismatch, inspect the stored graph and backup before retrying. Do not declare success from the HTTP response alone.

## Behavioral review

Review normal, false, and error paths; unreachable cards; mutually exclusive joins; cycles and flow-to-flow recursion; concurrent executions; manual overrides; missing or unavailable devices; time windows crossing midnight; daylight-saving transitions; and restart recovery. A validator checks only the constraints it implements. Test with the user's agreement and an explicit expectation for each branch. Never run physical actions merely to check JSON.

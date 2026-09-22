# Logic, variables, and tags

Homey Logic variables have exactly three types: `boolean`, `number`, and `string`. Their values persist across flow runs. Tags expose current values from variables, devices, apps, and trigger/action cards; a tag does not necessarily require a new persistent variable.

Use a boolean for a real binary state, a number for a measurement, count, target, or documented timestamp, and a string for text or a deliberately specified state enum. State the units for numbers. Avoid encoding numbers as strings or creating one boolean per mutually exclusive state when one clearly defined state value suffices.

## Decide whether state is needed

First inspect existing variables and relevant device capabilities. Read the actual temperature or on/off capability when that answers the question. Use a trigger token for data belonging to the current event. Use conditions and branches for transient decisions. Introduce a persistent variable only for cross-run memory or genuinely shared state, such as a manual-override expiry or last successful operation.

Do not reuse a variable merely because its name resembles the desired concept. Check its type, units, current consumers, and ownership. Changing an existing value can trigger flows. Avoid resetting runtime values during a deployment.

Prefer a namespace scoped to the approved automation, for example `AI.Heating.LivingRoom.OverrideUntil`. The prefix is a naming convention, not a Homey permission boundary. Keep names readable in the mobile editor. Include each variable's purpose and lifetime in the manifest and explain which flow writes it.

## Manifest and workflow

```json
[
  {
    "name": "AI.Heating.LivingRoom.ManualOverride",
    "type": "boolean",
    "value": false,
    "purpose": "Prevent scheduled heating changes while the user override is active"
  }
]
```

`variables batch manifest.json` is a read-only plan. `--apply` creates missing variables and reuses exact case-insensitive names with matching types. Reuse never writes the manifest's initial value over a current value. Duplicate names, ambiguous existing names, type mismatches, malformed values, and unknown manifest fields are rejected.

For 10+ new variables, provide a purpose for each and obtain approval of the exact plan. Apply using the returned hash and count. For 30+, also supply the agreed `--prefix`. A changed name, type, initial value, purpose, or set of new variables changes the approval hash. Current values of reused variables may change naturally without invalidating approval because the command does not write them.

The batch receipt records the plan, created IDs, and progress. There is no multi-object transaction. A network error can leave a partially applied plan; inspect and re-plan instead of assuming rollback.

## Referencing variables in cards

Discover Logic trigger, condition, and action definitions using `flows cards`. Read their arguments rather than assuming names such as `value` or `comparator`. An autocomplete variable argument commonly has this structure:

```json
{"variable":{"id":"<discovered-variable-id>","name":"AI.Heating.LivingRoom.ManualOverride"}}
```

For a global Logic droptoken use `homey:manager:logic|<variable-id>`. For an embedded tag in a text argument, the form commonly uses `[[homey:manager:logic:<variable-id>]]`. Preserve the distinction and check an exported working example when uncertain.

Keep number and boolean arguments typed in JSON. `"false"` is a string, not a boolean. Validate existing variable IDs with `flows validate --online` after creation and before deploying the flow.

## Avoid feedback and stale state

A variable-change trigger paired with an action that sets the same variable can create a loop. Guard on a meaningful transition, compare the old/new state where supported, or separate input state from derived state. Explain how multiple flows writing the same variable coordinate.

For timestamps, agree on units and timezone. For counters, consider concurrent increments and restart behavior. For occupancy delays, define how fresh motion invalidates a pending “off” action. Prefer installed timer capabilities when they provide cancellation instead of assuming a delay card has it.

Before removing a variable, inspect all ordinary and Advanced Flows and relevant HomeyScripts for its ID and embedded-tag references. No static scan can prove an arbitrary script never constructs the ID dynamically. Confirm the deletion target and its effect with the user. Recreated variables have new IDs.

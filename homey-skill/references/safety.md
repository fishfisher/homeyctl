# Approval, drafts, and recovery

Honor the user's existing authorization. Ask only when a choice changes the requested behavior, persistent state, or physical effect. Read-only discovery, local drafts, validation, and previews can proceed while awaiting a decision.

| Change | Agent behavior |
| --- | --- |
| Inventory, read existing flows, validate, local preview | Proceed within the task |
| New disabled flow in AI Flows, within an authorized build request | Explain design, create draft, report ID |
| 1–4 new variables | Explain purposes; proceed if included in the authorized design |
| 5–9 new variables | Present grouped plan; ask unless this exact scope is already approved |
| 10–29 new variables | Obtain explicit approval of manifest and count; use matching plan hash |
| 30+ new variables | Also agree on namespace, per-variable purposes, type counts, reuse, and simpler alternatives |
| Editing an existing flow | Stay within the approved change; mandatory automatic backup; preview substantial changes |
| Enabling or manually triggering a draft | Obtain authorization for its physical effects; disabled does not mean untriggerable |
| Deleting flows/variables or broad refactoring | Confirm exact targets and consequences; back up; inspect references |

Count variables across the complete user-visible operation, including multiple manifests and single-create commands. Never split a batch to avoid an approval threshold. The CLI's hash/count checks guard against mistakes; they do not prove that a human approved anything. An agent must actually obtain the approval before supplying confirmation flags.

Use a compact proposal, for example: “This design needs 12 new variables under AI.Heating: 4 override booleans, 4 target temperatures, and 4 expiry timestamps. It reuses 3 existing variables. Each new variable and its purpose is in this plan. May I create these 12?” Do not request vague blanket permission to create whatever may be needed.

## Backups

Updates and deletions of flows require a successful backup before the API write. The backup is the full fetched flow document, stored under the OS configuration directory's `homeyctl/backups` with private file permissions and a unique timestamp. On macOS this is `~/Library/Application Support/homeyctl/backups/`. Always report the backup path after changing an existing flow.

Keep backup documents private: names, arguments, URLs, and notes may contain personal information or integration secrets. Never commit live exports or receipts. Use redacted fixtures for tests.

A backup does not lock Homey or make writes atomic. Avoid concurrent agents writing the same flow. Refresh the complete flow immediately before changing it. For substantial redesigns, create a disabled draft copy and retire the original only after the user accepts the replacement.

## Recovery

Preview restoring an existing flow with `flows update <same-id> <backup.json> --dry-run`; use `--replace-cards` for Advanced Flows. Apply only when the intended restoration is clear. Restoration backs up the current version first. Review its `enabled` and `folder` fields: restoring an old backup can re-enable old behavior.

Recreating a deleted flow creates a new ID; repair dependent references explicitly. Recreating a deleted Logic variable also creates a new ID. Variable backups cannot transparently undo references to the deleted ID. Prefer preserving and reusing objects over deletion.

For variable batch failures, retain the receipt. Its `created` entries identify successful creations and `pending` identifies the request whose outcome may be uncertain. Refresh Homey before retrying. Re-planning reuses matching names/types and preserves their values. Ask before cleanup of persistent objects, especially if flows may already reference them.

On timeout after any write, do not retry automatically. First check whether the object or change already exists. Report uncertainty and the concrete IDs or receipt that help resolve it.

## Physical effects

Setting a Logic variable can fire other flows. Running an action card, HomeyScript, mood, or manual flow can change devices immediately. Creating a disabled draft does not authorize executing any of those operations. Treat locks, doors, alarms, heating, and other consequential controls according to the user's explicit intended outcome.

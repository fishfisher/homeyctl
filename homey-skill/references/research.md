# Sources and interoperability

The builder workflow combines local implementation checks, official Homey documentation, and read-only inspection of real exported flows. External examples are useful, but the installed cards and the target Homey's stored JSON determine the concrete IDs and shape.

- [Athom Homey CLI](https://github.com/athombv/node-homey): official CLI; its schema-driven API commands expose manager endpoints and are useful for schema inspection.
- [Homey Web API](https://api.developer.homey.app/): official API reference.
- [Homey Flow card arguments](https://apps.developer.homey.app/the-basics/flow/arguments): official app/card argument definitions.
- [Homey Logic guide](https://support.homey.app/hc/en-us/articles/4410240765586-Use-Logic-in-Flows): variables and tags.
- [Official Homey MCP announcement](https://homey.app/en-us/news/introducing-the-homey-mcp-server/): MCP endpoint and device/flow/mood interaction. Do not assume MCP tools support creating or revising Advanced Flow graphs merely because they can trigger them. Inspect the connected server's actual tools.
- [timvdhoorn/homey-cli-skill](https://github.com/timvdhoorn/homey-cli-skill): useful discovery, backup, canvas, and validation ideas. This package uses homeyctl-specific commands and independently written guidance.
- [KrauseFx/homey-cli](https://github.com/KrauseFx/homey-cli): useful reference for device capability validation and focused command design.

The official MCP server can complement this CLI for conversational discovery and control. Use one source of truth for IDs and the selected Homey. Keep flow drafts, manifests, backups, and receipts understandable outside any agent provider. This skill depends only on a shell and homeyctl; it does not require MCP or provider-specific APIs.

Observed compatibility details include optional null droptokens, bare local tokens in ordinary flows, existing custom canvas keys, and ANY joins without an explicit input array. These observations should become redacted regression tests when they affect validation. Do not include private live exports in the repository.

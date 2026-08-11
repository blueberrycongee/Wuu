# Extend Wuu

Wuu has four main extension paths: Skills, MCP, Hooks, and Wuu Plugins. They can work
together, but they solve different problems and carry different trust costs. Start
from the behavior you want to change rather than from an API name.

## Choose the smallest extension that fits

| What you want to do | Start with | Why |
| --- | --- | --- |
| Teach the agent a repeatable workflow | [Skill](skills.md) | Reusable instructions and resources, with no plugin process |
| Connect an existing local or remote tool service | [MCP](mcp.md) | Reuse a standard MCP server without changing Wuu |
| Run a check around tool calls or prompt submission | [Hook](hooks.md) | Fits policy checks, blocking, logging, and automation |
| Ship only a theme or host-rendered settings | Declarative [Wuu Plugin](plugins.md) contributions | No Desktop code needs to load |
| Register agent tools, context, or long-running behavior | [Wuu Plugin](plugins.md), Agent side | A managed runtime can join the agent lifecycle and call host services |
| Add desktop buttons, panels, pages, or message presentation | [Wuu Plugin](plugins.md), Desktop side | A desktop module can run React at stable UI boundaries |
| Ship Skills, MCP, Hooks, agent behavior, and UI together | [Wuu Plugin](plugins.md) | One package owns installation, approval, upgrade, and removal |

Prefer a Skill or MCP server when it fully solves the problem. Use a Wuu Plugin when
you need code lifecycle, host services, or desktop UI.

## How they compose

A code-review extension might include all of the following:

1. a Skill that defines the review steps and output format;
2. an MCP server that exposes internal code-search tools;
3. a Hook that runs a compliance check before completion;
4. a Wuu Plugin that adds a review-history View and dedicated agent tools.

Only a Wuu Plugin is a package with `plugin.json`, generation lifecycle, and approval
state. A plugin package may carry Skills, MCP, and Hooks, but those contributions are
not themselves desktop modules.

## Runtime and trust

| Extension | Where it runs | Main trust concern |
| --- | --- | --- |
| Skill | Instructions in agent context | May direct the agent to use tools; read it first |
| MCP | Local subprocess or remote server | Local commands, network access, and third-party tool output |
| Hook | Local command or model call | Runs during lifecycle events and may block or rewrite behavior |
| Agent plugin | Wuu-managed subprocess | Runs with user authority and may register tools or call approved services |
| Desktop plugin | Trusted code in the Wuu Renderer | Runs React and may inject CSS; install only trusted sources |

Wuu permission modes, workspace boundaries, and plugin approval still apply. A smaller
extension is easier to reason about, not exempt from source review.

## Start building

- [Use and write Skills](skills.md)
- [Write and install a Skill](skill-authoring.md)
- [Connect MCP servers](mcp.md)
- [Configure Hooks](hooks.md)
- [Understand Wuu Plugins](plugins.md)
- [Use plugin themes and settings](themes-settings.md)
- [Agent plugin quickstart](plugin-quickstart.md)
- [Desktop plugin quickstart](desktop-plugin-quickstart.md)
- [Desktop UI extension map](desktop-plugins.md)
- [Desktop plugin recipes](plugin-recipes.md)

Use the [plugin authoring reference](plugin-authoring.md) for complete fields,
lifecycle, and APIs. Read the [plugin system architecture](plugin-system.md) last when
you need the design rationale behind those boundaries.

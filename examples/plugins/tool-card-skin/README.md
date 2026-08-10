# Tool Card Skin

Tool Card Skin is the second appearance-composition prototype. It replaces the
`command.bash` Tool Activity Presenter while using only the immutable public
`ToolActivitySnapshot` and the host fallback.

The plugin deliberately does not parse Tool arguments, inspect Agent steps, or
read Goal, Subagent, Automation, Provider, or Loop state. The native fallback
remains mounted inside the card, so host-owned output, actions, streaming, and
error behavior stay available. Disable the plugin to remove the Presenter and
its generation-scoped CSS together.

Use it alongside Manga Studio to verify that a Theme/Token/snippet plugin and a
Presenter plugin compose independently.

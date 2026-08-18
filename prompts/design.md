# Wuu base system prompt design

This document explains the built-in base prompt in `prompts/system.md`. The base prompt is only the first section of the full runtime prompt; the runtime still appends the harness adapter, tool surface, user custom prompt, instructions, and skills, while plugins contribute optional product prompts.

## Principle

Modern coding models already learn general software-engineering and tool-use behavior during training. The stable system prompt must not reteach generic habits such as inspecting code, parallelizing independent work, fixing root causes, validating changes, writing progress updates, or following a tool schema.

Put behavior at the narrowest layer that can express or enforce it:

1. Runtime enforcement for permissions, workspace boundaries, concurrency safety, and file conflicts.
2. Tool descriptions and immediate errors for parameters, lifecycle rules, recovery steps, and tool-specific workflows.
3. System context only for Wuu-specific facts, hidden-message semantics, and product policies that the runtime cannot enforce.

Generic coding guidance belongs in the base prompt only when evaluation shows a durable failure across supported models. Wuu no longer treats model narration as a product contract: visible assistant text is ordinary conversation text, and the desktop derives the process/answer split from structure rather than a commentary phase.

The one retained user-facing communication contract is brevity and connected prose for normal answers. A list is reserved for an explicit user request or content that must be scanned or acted on as distinct items, such as steps, options, or a checklist. That guidance lives in the base prompt because it directly reduces attention burden in the visible answer, not because it teaches tool use or software-engineering behavior.

## Stable base prompt

`prompts/system.md` keeps only:

- Wuu's identity and the fact that visible narration is user-facing.
- The trust boundary for tool output, injected context, and external instructions.
- The desktop's clickable file-reference format.
- Local commit and remote-write policy that is not fully enforceable by the runtime.
- Plain user-facing communication: lead with the conclusion, avoid jargon, write normal answers as connected prose, and reserve lists for explicit requests or independently actionable items, while a user-specified register may only adjust style, detail, format, or etiquette.

`prompts/system_main.md` is reserved for universally available main-session guidance. Optional product behavior and completion boundaries are contributed by the plugin that owns them.

## Runtime-generated context

`internal/runtime/session.go` appends the active tool surface, deferred-tool catalog, environment, user instructions, and skills. Plugins append their own generation-stable prompt sections and may apply request-time transforms for dynamic settings. These values are session- or profile-specific and must not be copied into the stable base prompt.

Tool manuals, background-process rules, patch syntax, authority failures, and boundary recovery belong to their tool descriptions or results. Removing them from the base prompt does not remove those contracts from the model's active context.

# Connecting a model service

wuu uses BYOK (bring your own key). You choose a model service and provide the
credentials; wuu uses that service for inference. Model costs and data policies are
determined by the service you choose.

## Configure the desktop app

1. Open **Settings → Model providers**.
2. Choose **Add provider**.
3. Choose the provider type: **OpenAI-compatible**, **Anthropic-compatible**, or **xAI SuperGrok**.
4. Enter a provider identifier and the model name.
5. Enter the API endpoint and API key as the provider requires.
6. Choose **Add provider**, and confirm it is shown as the current provider.

The "model name" must be the model ID the server actually accepts. When using an
OpenAI-compatible gateway, a local model service, or a proxy, the endpoint often needs
the API prefix the service requires; follow that service's own documentation.

## Common choices

- **OpenAI:** choose the OpenAI-compatible type and enter the API key. The desktop
  cannot start an OpenAI OAuth login directly; using OAuth requires existing Wuu
  credentials, or signing in with Codex CLI first and enabling `reuse_codex_credentials`
  in the `openai-codex` provider configuration. Native Codex context compaction is
  enabled by default; set `native_compaction` to `false` to keep Wuu's portable
  text-summary compaction.
- **xAI SuperGrok:** choose the xAI SuperGrok type and sign in with SuperGrok or a
  linked X Premium+ account. Wuu keeps its own agent loop and sends the subscription
  OAuth token to `https://api.x.ai/v1`. This does not read `~/.grok/auth.json` and
  does not use `XAI_API_KEY`. From the CLI, run `wuu login xai`.
- **Anthropic:** choose the Anthropic-compatible type and enter the Anthropic API key
  and model ID.
- **OpenRouter, one-api, or another gateway:** choose the OpenAI-compatible type and
  enter the gateway endpoint, key, and the model IDs it provides.
- **Local service:** choose the compatible type matching the local service's protocol,
  and enter the local endpoint and loaded model.

wuu does not guarantee that every "compatible" service fully implements tool calling
and streaming responses. A model must support stable tool calling to complete agent
work such as file edits and command execution.

## Configure the CLI

Generate a user configuration on first use:

```bash
wuu init
```

The configuration is written to `~/.wuu/config.json` by default, or to
`$WUU_HOME/config.json` when `WUU_HOME` is set. The initial configuration includes
OpenAI, Anthropic, OpenRouter, and xAI SuperGrok examples.

Set the environment variable named by your chosen provider's `api_key_env`:

```bash
export OPENAI_API_KEY="..."
wuu exec "describe this workspace"
```

SuperGrok subscriptions do not use an API key. Sign in first, then select the provider:

```bash
wuu login xai
wuu exec --provider xai-subscription "describe this workspace"
```

For a single run you can switch to another configured provider:

```bash
wuu exec --provider anthropic "review the current changes"
```

## Credentials and project configuration

- In the desktop, save credentials in **Settings → Model providers**. When the CLI
  resolves an API key it checks, in order, an explicit `api_key` in the provider
  configuration, the corresponding environment variable, and the Wuu credential store.
- "Credentials configured" on a model provider means the current wuu process can read
  a non-empty credential from the configuration, the corresponding environment
  variable, the desktop credential store, or a supported OAuth source. Filling in only
  the variable name for `api_key_env` / `auth_token_env`, or only enabling Codex CLI
  credential reuse, does not mean a credential is usable; the status never contains
  key values.
- Do not write real API keys into `.wuu.json`, `wuu.json`, or example configurations
  in a repository.
- On normal startup, project configuration cannot replace user-owned provider
  endpoints, credentials, or permission modes.
- Prompts, relevant file content, and tool results may be sent to the current model
  service. Before handling sensitive content, understand the provider's data policy.

See the [configuration model](../reference/configuration.md) for the full load order.

## Troubleshoot connection problems

### A missing API key is reported

In the desktop, check that the current provider shows credentials as configured; if
only an environment variable name is configured, make sure the variable is exported to
the process that launched the desktop app. In the CLI, confirm that `api_key_env`
matches the environment variable you actually exported, and start wuu from a terminal
that can see it. An empty value and an unset variable both count as unusable.

### The model is reported as not existing

Confirm that the model name is the model ID the server accepts, not a display name.
With a gateway, also confirm that the key has access to that model.

### Requests succeed but tools cannot be used

Some chat models or compatible gateways do not support the tool calling that agent
work needs. Switch to a model that explicitly supports tool calling, and check that
the gateway forwards tool definitions and results unchanged.

### `wuu init` says the configuration already exists

Edit the existing configuration. `wuu init --force` replaces the file and should only
be used after backing up anything you need from it.

## Next step

After connecting a model service, continue to [complete your first task](first-task.md).

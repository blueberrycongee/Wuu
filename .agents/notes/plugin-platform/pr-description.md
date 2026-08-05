# PR 描述草案（feat/plugin-platform → main）

> 供 Andy 发 PR 时采用/删改。由安全轨基于分支 16 个提交（61 文件，+4783/−217）起草。

---

## Summary

Build the Wuu plugin platform: packages discovered from `~/.wuu/plugins`, project `.wuu/plugins`, and bundled sources stay inert until policy activates them; a closed permission catalog is derived from each package's declared surfaces and enforced at both the package gate and the protocol-hook boundary; the desktop gains an Extensions catalog and plugin-contributed slash commands. Reference design: Cordis service composition, adapted to Wuu's subprocess model so permissions are actually enforced rather than advisory.

## What changed

**Approval platform**
- Discovery is inert; community packages activate only with an exact-fingerprint user grant. Fingerprint covers manifest, executable args, hooks, commands, prompt assets, and skill trees. Official bundled packages are trusted by provenance and cannot be shadowed by user/project ids.
- Grant/reject/disable are user-owned; project/shared settings cannot set package policy.
- Revoke or fingerprint change closes the plugin process and removes all its contributions (runtime reconcile + `Host.Clients/Replace`).

**Closed permission catalog + enforcement**
- 15-permission closed catalog with alias normalization; unknown values fail validation.
- Host derives required permissions from declared surfaces (runtime → `process.spawn`, MCP → `process.spawn`/`network.connect`, prompt hook → `session.read`+`session.write`); activation requires the grant to cover the full union.
- `wuu-plugin-v1` protocol hooks are independently mapped to permissions (`chat.request` → `session.read`+`session.write`, `tool.*` → `tools.define`/`tools.intercept`, `shell.env` → `shell.env`); hooks declared without grants are stripped at initialize and surfaced as diagnostics (`Status.StrippedHooks`).
- Plugin processes get a minimal documented environment — ambient API keys no longer leak through `os.Environ` inheritance.

**Desktop**
- SkillsCatalog evolves into an Extensions management surface (approve/reject/enable/disable, diagnostics, contributions).
- Plugin-contributed commands flow manifest → Go → protocol/preload → slash registry (`PluginCommandRegistry`).

**Docs**
- `docs/{en,zh-cn}/reference/security-model.md` gains a Plugin packages section with the trust model and residual-risk disclosure.
- `.agents/notes/plugin-platform/` carries the design record: architecture, threat model, permission catalog, security audit, integration verification, pre-review.

## Test plan

- New adversarial coverage: project/user shadowing of bundled ids, grant revoke closes client, fingerprint change restarts runtime, env secret non-inheritance, unknown-hook rejection, oversize responses, permission catalog validation/derivation, protocol-hook stripping (partial and zero grant), resolver semantics (official/community/missing).
- `go test ./internal/{plugin,pluginhost,extensions,runtime,appserver}/...` green; full-repo `go test ./...` green except noted in-flight items below; desktop targeted vitest 158/158.
- Verification record: `.agents/notes/plugin-platform/integration-verification.md` (includes pure-HEAD build gate — every commit compiles in isolation).

## Residual risk (honest)

A granted plugin is native code running as the current OS user. Environment filtering, the closed catalog, and hook stripping prevent accidents and unapproved capabilities; they are not an OS sandbox. `network`/`files.*` permissions are declarative in v1. Future: Seatbelt/Landlock/AppContainer adapters, WASM capability runtime, signed package indexes, resource limits.

## Notes for reviewers

- Two commits (`7235ae8b`, `63adb873`) repair the committed build after an early partial commit; details in the integration record.
- Config settings-layer change (`protectUserSettings`) must preserve the local-layer exemption for extension grants; covered by `TestLoadFrom_LocalSettingsCarriesExtensionGrants`.

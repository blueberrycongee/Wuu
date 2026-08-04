# Plugin platform threat model

## Security promise

Wuu can enforce discovery-versus-activation, approval integrity, host API permissions, payload minimization, environment filtering, lifecycle isolation, and renderer separation. Wuu cannot claim that a native subprocess running as the same OS user is unable to read user-readable files or open the network without an additional OS sandbox.

## Assets

- provider credentials and environment secrets;
- session messages, attachments, memory, and user configuration;
- workspace and home-directory files;
- shell/tool authority and approval decisions;
- renderer preload APIs and Electron main-process capabilities;
- plugin grants, fingerprints, provenance, and update state;
- host availability, latency, and durable data integrity.

## Adversaries

### Malicious package

Attempts to exfiltrate session content or secrets, rewrite model/tool requests, execute commands, impersonate another plugin, escalate through renderer APIs, or persist after disable.

### Buggy or runaway package

Deadlocks a hook, emits invalid payloads, crashes repeatedly, leaks processes, consumes unbounded output/memory, or leaves stale registrations after reload.

### Supply-chain compromise

A project repository introduces `.wuu/plugins`; an installed package changes executable content; an update requests new authority; a package shadows a trusted id; or an external executable changes behind an unchanged manifest.

## Required controls for the first PR

1. Project/user discovery is inert until exact package approval.
2. Official trust derives from bundled provenance, never manifest text or id.
3. Aggregate fingerprint changes stop activation and invalidate the grant.
4. Package id collisions have deterministic precedence; bundled provenance cannot be shadowed by user or project packages.
5. Unknown executable manifest fields and permissions fail closed.
6. Plugin processes receive a minimal documented environment, not the host environment.
7. Sensitive hooks require declared and granted permissions.
8. Host-owned connections establish plugin identity; caller-supplied ids grant no authority.
9. Hook/action calls have deadlines, cancellation, response-size limits, schema validation, and user-safe diagnostics.
10. Disable/revoke/change removes all package-owned contributions and terminates its runtime.
11. Declarative UI is host-rendered and cannot access preload or raw DOM/CSS capabilities.
12. Tests include malicious fixtures for environment access, undeclared hook registration, invalid output, timeout, fingerprint change, project auto-start, identity spoofing, and stale registrations.
13. Shared/project-authored configuration cannot grant, reject, disable, or otherwise set user-owned package policy.
14. Fingerprints cover executable argument order, hook execution options, declarative commands, prompt assets, and complete skill asset trees.
15. Package-relative assets are checked after symlink resolution and cannot escape the package root.

## Permission behavior

Permissions are a closed catalog. A package requests permissions; the user approves the exact aggregate request; the runtime host independently maps each surface to required permissions. Manifest declaration alone never authorizes behavior.

Read and write authority are separate where the payload contract permits it. Observation hooks receive immutable or minimized payloads. Transform hooks require write authority. A plugin cannot obtain session content merely because it receives lifecycle metadata.

## Residual risk disclosure

Granted native executables can use ordinary same-user filesystem and network APIs outside the Wuu RPC boundary. Environment filtering prevents accidental secret inheritance but is not a native-code sandbox. The UI must label this tier “trusted executable,” display provenance and executable path, and explain that approval grants code execution as the user.

## Future controls

- platform sandbox adapters for Seatbelt, Landlock/bubblewrap, and Windows AppContainer/restricted tokens;
- WASM runtime with capability-based host calls;
- signed package indexes and reproducible bundle hashes;
- resource limits, crash backoff, and audit log export;
- scoped capability handles for sandboxed renderer frames.

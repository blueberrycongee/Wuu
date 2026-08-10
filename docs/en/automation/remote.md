# Remote devices and Relay

wuu's remote capability lets one computer act as a Host and provide remote control to
paired phones or other clients through a self-hostable Relay. The Relay only forwards
encrypted connections; the Host still runs wuu in the designated workspace, and the
model configuration, permissions, and file-access boundary remain decided by the Host.

## Initialize the Host

First create a remote identity for the current user. Remote state is saved to
`~/.wuu/remote.json` by default; with `WUU_HOME` set it moves together with the user
directory.

```bash
wuu remote init --relay wss://relay.example.com/v1/connect --name my-mac
```

`--relay` can be omitted and added later by running the init command again; the command
prints the remote identity fingerprint, which you can use during pairing or
troubleshooting to confirm you are connecting to the right Host.

## Start the Host and pair a phone

Start the Host on the computer you want to control remotely:

```bash
wuu remote host --workdir /path/to/project --pair
```

The terminal prints a pairing URI. Turn the URI into a QR code or copy it to the phone;
the pairing window lasts 10 minutes by default and closes automatically after the first
device pairs. To adjust the behavior:

```bash
wuu remote host --pair --pair-timeout 30m --pair-once=false
```

You can also override the provider, model, workspace, or Relay when starting the Host:

```bash
wuu remote host \
  --workdir /path/to/project \
  --provider openai \
  --model gpt-5.6 \
  --relay wss://relay.example.com/v1/connect
```

The Host process must keep running; stopping the Host or Relay does not delete saved
pairing identities.

## Manage paired devices

View the Host identity, Relay, and device count:

```bash
wuu remote status
wuu remote status --json
wuu remote devices
wuu remote devices --json
```

Revoke a device using its displayed fingerprint:

```bash
wuu remote devices remove <fingerprint>
```

Revoking blocks new handshakes; a running Host may need a reload or restart before it
fully adopts the latest device list. Do not publish full public keys, pairing URIs, or
`remote.json` to issues or chats.

## Phone-side commands

The phone side first imports the pairing URI:

```bash
wuu remote phone pair --uri 'wuu://pair?...'
```

Then you can view connection status, send tasks, and watch events:

```bash
wuu remote phone status
wuu remote phone send "check the test status of the current workspace"
wuu remote phone watch
```

The phone-side state file lives in the user directory by default; to isolate multiple
identities, use `--store FILE` to point at a separate file. The permissions for sent
tasks are still decided by the Host's runtime and workspace policy; the phone cannot
bypass read-only mode or sensitive-path protections.

## Troubleshooting

- **Cannot connect to the Relay:** check the `ws://`/`wss://` address, port, firewall,
  and the Relay's `/v1/connect` path; the Host and phone must use the same reachable
  Relay.
- **Pairing URI expired:** restart `wuu remote host --pair` to generate a new window
  and URI.
- **Device shows paired but cannot operate:** run `wuu remote status` first, then
  confirm the Host process is still running, the workspace exists, and the model
  service credentials are available.
- **A task starts but the result is interrupted:** look at the phone's `watch` output
  and the Host logs; do not resend the same task, first confirm whether the previous
  run is still in progress.

When the remote feature involves public-network connections, configure TLS, access
control, and log redaction for the Relay; do not treat the Relay as a model service or
credential store.

## Related documentation

- [Model services](../getting-started/model-services.md): configure the provider and
  model the Host uses;
- [Permission modes](../reference/permissions.md): which paths and commands the Host
  can read and write;
- [App-server integration](app-server.md): use the core protocol when building other
  clients.

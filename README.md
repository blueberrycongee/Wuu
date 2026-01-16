# Wuu

Early-stage repository for the Wuu language spec and a minimal Rust toolchain prototype.

- Spec: `docs/wuu-lang/SPEC.md`
- Examples: `docs/wuu-lang/EXAMPLES.md`
- Self-host plan: `docs/wuu-lang/SELF_HOST_PLAN.md`
- Progress log: `docs/PROGRESS.md`

## Tooling (prototype)

Build and run (use WSL on this machine):

```sh
cargo run -- fmt path/to/file.wuu
cargo run -- check path/to/file.wuu
```

### WSL + D: drive caches (this machine)

One-time:

```powershell
.\scripts\wsl-bootstrap-rust.ps1
```

## Autoloop

See `docs/AUTOLOOP.md` and `prompt.md`.

## License

Apache-2.0. See `LICENSE`.

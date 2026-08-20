# Vendored Blobatar

Wuu vendors Blobatar because the mascot is rendered at 24–32 px and needs a
larger shared eye footprint and coherent face-sphere perspective than upstream
`v0.2.0` provides.

- Upstream: https://github.com/Alain00/blobatar (MIT, see `LICENSE`)
- Baseline: `v0.2.0`, plus Wuu changes: enlarged shared eye geometry, and a
  `perspective` option that projects the eye pair onto a turned sphere —
  positions, foreshortening, surface rotation, and a per-path bend, so the
  eyes read as painted on a ball rather than taped to a disc.

This is vendored **as source**, not as a tarball: `package.json`'s `exports`
point at `src/*.ts` and the desktop renderer bundles the TypeScript directly
through the `file:vendor/blobatar` dependency. There is no dist and no build
step here — editing `src/` and restarting the desktop dev build is the whole
integration loop.

## Layout

- `src/` — the library. `test/` — its bun test suite (`bun test`).
- `demo/` — the tuning grid playground. `bun install && bun dev` in there,
  then open http://localhost:3001/ to tune traits, expressions and the sphere
  perspective against a grid of seeds.
- `docs/` — design notes and ADRs. `CONTEXT.md` — the glossary; worth reading
  before changing anything.
- `scripts/probe-compose.ts` — manual browser gate (`bun run probe`, needs
  Chrome or Firefox) that checks a statically baked pose agrees with the CSS
  composition. Run it after touching `expression.ts` or `motion.css`.

The desktop's own typecheck and unit tests cover the import path end to end.

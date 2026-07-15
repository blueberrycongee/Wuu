# Changelog

This file starts the maintained release record. Earlier GitHub Releases may not
have complete change notes.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning rules are documented in [the release guide](docs/release.md).

## [Unreleased]

### Changed

- Release versions are synchronized from `VERSION`, validated in CI, and
  published with the matching changelog section.
- The npm wrapper installs the GitHub Release matching its own package version
  instead of silently resolving the latest release.

## [0.1.0] - 2026-07-10

### Added

- Unsigned macOS arm64 Electron desktop preview DMG/ZIP artifacts on GitHub
  Releases
- `LICENSE` (MIT) so the project is unambiguously open source
- `CONTRIBUTING.md`, `SECURITY.md`, and `CODE_OF_CONDUCT.md` for open-source
  governance
- `CHANGELOG.md` to track user-visible changes
- `.github/CODEOWNERS`, `.github/ISSUE_TEMPLATE/` (bug report, feature
  request), and `.github/PULL_REQUEST_TEMPLATE.md`
- `.gitignore` entries for `coverage/`, `*.log`, `.idea/`, `.vscode/`, and
  `.env*` environment files

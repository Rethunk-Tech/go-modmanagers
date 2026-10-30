# Changelog

Notable changes to `go-modmanagers`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-10-30

### Added

- `r2modman.WriteExport` builds an `.r2z` body that `ReadExport` reads back, and `r2modman.ParseVersion` reads a `major.minor.patch` string into a `Version`.

## [0.1.0] - 2026-10-29

### Added

- `gale`: list the profiles in Gale's `data.sqlite3` read-only, parse a profile's mods on demand, read a sync code from pasted text (id, copied sync link or `gale://` deep link) and download its `.r2z`.
- `r2modman`: read `.r2z` exports and pasted profile codes (including Gale's `gale://profile/import/` link) through go-archive's admission rules, fetch a code from Thunderstore, read profile folders (`mods.yml`, `BepInEx/config`) and list the manager's data folders per OS, the host's never the Flatpak sandbox's own.
- `vortex`: read Vortex's `state.v2` from a private copy (profiles, mods, staging folder, multi-user flag, data-folder candidates); a database a running Vortex holds open is `ErrLocked`.
- `stardrop`: list and read Stardrop profiles (both enabled-mod shapes, preserved configs, Nexus ids) and resolve its mods folder from `Settings.json`.
- `mo2`: open any Mod Organizer 2 instance folder with its configurable mods and profiles folders resolved, list profiles, read `modlist.txt` and `meta.ini`.

[Unreleased]: https://github.com/Rethunk-Tech/go-modmanagers/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Rethunk-Tech/go-modmanagers/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Rethunk-Tech/go-modmanagers/releases/tag/v0.1.0

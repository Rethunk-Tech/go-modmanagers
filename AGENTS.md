# AGENTS.md

This module reads the files other mod managers keep their profiles in, one package per format: `vortex`, `mo2`, `stardrop`, `r2modman`, `gale`. It returns each manager's own records and resolves nothing into another program's model, and it never writes the files it reads (`r2modman.WriteExport` only builds a share-code body in memory). It imports no consumer package and has no root package, so there is no `doc.go`.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH. The `windows-latest` test job is the only place
Windows-only behaviour runs: `vortex`'s `TestReadReportsLocked` and the readers' backslash and drive-letter paths.

## Shape

- One package per format, no shared type or interface: the managers' records differ, and a common profile type would be the lowest common denominator a caller then has to undo. A package imports only its own format's third-party dependencies (LevelDB only in `vortex`, SQLite only in `gale`, YAML only in `r2modman`), so a caller of one does not link the others.
- Nothing writes the source. `vortex.Read` works on a private copy, because a running Vortex holds `LOCK` and a copy guarantees the live files are never opened for writing; `gale.Profiles` opens SQLite read-only and immutable for the same reason.
- `.r2z` bytes are read through go-archive's `ExtractFunc`, so a profile code is never admitted more loosely than any archive: a second hand-rolled zip reader would drift from those rules.
- Every read of input another program wrote is capped (`safefs.ReadCapped`, `httpkit.DoCapped`), and an over-cap error wraps `safefs.ErrTooLarge`.
- Folders come from the caller, except `Bases`, the one per-OS answer: on Linux it uses `userdirs.HostConfigHome` or `HostDataHome`, so inside a Flatpak the manager's folder is the host's and the sandbox's own folder, which holds no other manager's data, is never searched.
- The module reads the other managers' files and never writes them. The one writer is `r2modman.WriteExport`, which builds an `.r2z` body from an `Export` for a caller to share; it never touches a manager's folders.
- `gale` opens SQLite itself through CGO-free `modernc.org/sqlite` and takes no `*sql.DB`, because every caller that
  reads Gale needs a SQLite driver anyway.
- Fixtures are synthetic, built in the test or hand-written. No file another manager wrote is checked in: the
  upstreams are GPL-3.0, and real exports hold personal data. A real export used as a fixture is scrubbed first: for a
  Gale `data.sqlite3`, empty `auth`, `prefs` and `telemetry`, point `profiles.path` and each local mod's `icon` at
  neutral folders, then `VACUUM` and commit it checkpointed, with no `-wal` or `-shm`; for an `.r2z` or `mods.yml`,
  check `icon:`, paths and config values for personal data.
- The mods column of a Gale row is parsed by `Profile.Mods`, not by `Profiles`, so one malformed row does not hide the others.
- A path handed back to a caller is a native path, except `r2modman.File.Path` and `Export.Files` keys, which are slash-separated on every OS.

## Files

| File | Holds |
| --- | --- |
| `vortex/vortex.go` | `Read`, the LevelDB copy and tree rebuild, profiles, mods, staging folder, `Roots` |
| `vortex/locked_*.go` | Which error means another process holds a file open |
| `mo2/mo2.go` | Instance folders, `ModOrganizer.ini` paths, modlist, `meta.ini` |
| `stardrop/stardrop.go` | Profile files and `Settings.json` |
| `r2modman/export.go` | `.r2z` reading and its admission, `WriteExport`, `ParseVersion` |
| `r2modman/code.go` | Profile codes and the Thunderstore client |
| `r2modman/profile.go` | Profile folders, `BepInEx/config`, data folders |
| `gale/gale.go` | The profiles table, data folders, mods JSON |
| `gale/code.go` | Sync-code ids and the sync client |
| `internal/managerbase` | The per-OS base folder shared by `r2modman` and `gale` |
| `internal/nexusurl` | The Nexus mod id in a mod-page URL |

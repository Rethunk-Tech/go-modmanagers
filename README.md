<h1 align="center">go-modmanagers</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Read-only readers for the files other mod managers keep their profiles in: Vortex, Mod Organizer 2, Stardrop, r2modman and Gale. Module `github.com/Rethunk-Tech/go-modmanagers`, one package per manager.

Each reader returns the manager's own records in the manager's own terms (names, version text, enabled flags, folders, Nexus ids) and never writes, locks or resolves them into another program's model. A package imports only its own format's dependencies.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-modmanagers
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- `vortex`: profiles, mods and staging folder from `state.v2`, read from a private copy.
- `mo2`: instance folders, `[Settings]` paths, `modlist.txt` and `meta.ini`.
- `stardrop`: profile JSON and the mods folder from `Settings.json`.
- `r2modman`: `.r2z` exports and profile codes read under go-archive's admission rules, profile folders, data folders per OS.
- `gale`: the profiles table read-only, and sync codes from an id, a copied link or a `gale://` deep link.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.

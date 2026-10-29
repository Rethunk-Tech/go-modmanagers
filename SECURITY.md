# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

Every file here was written by another program and every code was pasted by a user or served over the network.

- `r2modman.ReadExport`: a hostile `.r2z` must not yield a path that leaves the archive, a link or special file, names that collide by case, a drive-letter name, or more than the entry, per-file and total caps; a malformed `export.r2x` must not panic.
- `r2modman.ReadProfile`, `Configs`: a file over 16 MiB is refused, not read whole.
- `r2modman.DecodeCode`, `CodeKey`, `gale.CodeID`: arbitrary pasted text must decode or fail, never panic.
- `r2modman.Client.FetchCode`, `gale.Client.FetchCode`: a response over `MaxBytes` is refused while reading, not held whole.
- `gale.Profiles`, `Profile.Mods`: a crafted database row or mods column must not panic or hide other rows.
- `vortex.Read`: a crafted LevelDB value must not panic, and the live database is never opened for writing.
- `mo2.Open`, `mo2.ParseModlist`, `stardrop.ReadProfile`: crafted INI, modlist or JSON must not panic.

## What is out of scope

The caller is trusted to decide which folders to read and what to do with the names and paths returned: a mod name is returned as written, not validated, and `gale.Profile.Path` and a Vortex mod folder are not checked to lie inside any root. SQLite's and LevelDB's own parsers are their projects' to secure. A file another manager keeps open for writing may read stale; that is intended.

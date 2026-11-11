# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository: <https://github.com/Rethunk-Tech/go-modmanagers/security/advisories/new>, or the **Security** tab, then **Report a vulnerability**. The report stays private between you and us until a fix ships.

If private reporting is unavailable, email [oss@rethunk.tech](mailto:oss@rethunk.tech) with **[SECURITY]** in the subject.

Please do not open a public issue, pull request or discussion for a suspected vulnerability.

A useful report has the affected repository and version (or commit), steps to reproduce, what an attacker gets and under which conditions, and whether anyone else knows.

## What counts as a vulnerability

A vulnerability is a flaw that lets someone cross a boundary they should not: run code, read or change data they have no right to, escape a sandbox or path restriction, or leak a credential. A bug is anything else that misbehaves, such as a crash on your own input, wrong output or a slow path; open an ordinary issue for those. If you are unsure which one you have, report it privately and we will say.

## Coordinated disclosure

We handle the report inside the private advisory, so you see each status change and can comment on the fix. The advisory goes public when the fix ships, or 90 days after your report, whichever comes first. If a fix is close, either of us can ask for more time in the thread. You are credited in the advisory unless you would rather not be.

## Response timelines

- We acknowledge your report within 3 business days.
- We give you an assessment (accepted, needs more information or declined, with reasons) within 10 business days.
- We aim to ship a fix within 90 days of the report.

## What is in scope

Every file here was written by another program and every code was pasted by a user or served over the network.

- `r2modman.ReadExport`: a hostile `.r2z` must not yield a path that leaves the archive, a link or special file, names that collide by case, a drive-letter name, or more than the entry, per-file and total caps; a malformed `export.r2x` must not panic.
- `r2modman.ReadProfile`, `Configs`: a file over 16 MiB is refused, not read whole.
- `r2modman.DecodeCode`, `CodeKey`, `gale.CodeID`: arbitrary pasted text must decode or fail, never panic.
- `r2modman.Client.FetchCode`, `gale.Client.FetchCode`: a response over `MaxBytes` is refused while reading, not held whole.
- `gale.Profiles`, `Profile.Mods`: a crafted database row or mods column must not panic or hide other rows.
- `vortex.Read`: a crafted LevelDB value must not panic, and the live database is never opened for writing.
- `mo2.Open`, `mo2.ParseModlist`, `stardrop.ReadProfile`: crafted INI, modlist or JSON must not panic, and an id or profile name must not be a path out of its folder. Each read of another manager's file is capped (16 MiB; 64 MiB for a Vortex database file).

## What is out of scope

The caller is trusted to decide which folders to read and what to do with the names and paths returned: a mod name is returned as written (except a modlist name that is a path, which `mo2` drops), and `gale.Profile.Path` and a Vortex mod folder are not checked to lie inside any root. SQLite's and LevelDB's own parsers are their projects' to secure. A file another manager keeps open for writing may read stale; that is intended.

## Safe harbour

We will not pursue legal action against good-faith research that avoids privacy violations, data destruction and disruption of services, stops at a proof of concept, and keeps the details private until the advisory is published. Rethunk.Tech, LLC maintains this repository.

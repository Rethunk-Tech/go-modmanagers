# HUMANS.md

Run and use `go-modmanagers`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-modmanagers
```

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Rethunk-Tech/go-modmanagers/gale"
	"github.com/Rethunk-Tech/go-modmanagers/r2modman"
)

func main() {
	ctx := context.Background()

	// A pasted Gale sync code, link or gale:// deep link becomes an .r2z.
	id, ok := gale.CodeID("gale://profile/sync/clone/e5rg8f")
	if !ok {
		log.Fatal("not a Gale code")
	}
	zip, err := (gale.Client{UserAgent: "example/1"}).FetchCode(ctx, id)
	if err != nil {
		log.Fatal(err)
	}
	export, err := r2modman.ReadExport(zip)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(export.ProfileName, len(export.Mods), "mods")
}
```

The other packages read local files: `vortex.Read(dataDir, vortex.Options{})` then `State.Profiles`, `mo2.Open(dir)` then `Instance.ReadModlist`, `stardrop.ReadProfile(stardrop.DataDir(appData), id)`, `gale.Profiles(ctx, gale.Database(dir))`.

## Contracts

| Call | Contract |
| --- | --- |
| `vortex.Read` | Copies `state.v2` (tables first, `CURRENT` last, never `LOCK`) and reads the copy; the live files are never opened for writing. A held-open database is `ErrLocked`; a missing one matches `fs.ErrNotExist` |
| `vortex.State.Profiles`, `Mods` | Game ids compare case-insensitively; `Profiles("")` lists every game; a nested profile with no `gameId` is skipped |
| `vortex.State.StagingFolder` | Expands `{USERDATA}`, `{USERNAME}`, `{GAME}` case-insensitively, accepts backslashes, resolves a relative result against the data folder |
| `mo2.Open` | Resolves `base_directory`, `mod_directory`, `profiles_directory` and `%BASE_DIR%` as MO2 does; relative paths resolve against the folder given; a missing ini is an error |
| `mo2.ParseModlist` | File order; `*` and `_separator` entries, blanks and comments skipped; a repeated name keeps its first place and last flag |
| `stardrop.ReadProfile` | `EnabledModIds` as strings or `{UniqueId}` objects, names trimmed, the file stem when the name is empty |
| `r2modman.ReadExport` | Refuses traversal, links, special files, names holding `:`, case-only collisions, exact duplicates, over 5,000 entries, a file over 16 MiB, 256 MiB in all; an absent `enabled` is enabled |
| `r2modman.CodeKey`, `DecodeCode` | A bare UUID or `gale://profile/import/<uuid>` is a key; a code is the prefix then base64 with any whitespace |
| `r2modman.Client.FetchCode`, `gale.Client.FetchCode` | A non-200 answer is a `*httpkit.StatusError`; a body over `MaxBytes` is `safefs.ErrTooLarge`; the deadline is the context |
| `gale.CodeID` | A six-character code in either case (returned upper-case), a 22-character base64url id unchanged, or a copied link or deep link ending in one |
| `gale.Profiles` | Opens the database read-only and immutable; a missing database lists nothing; the mods column is parsed only by `Profile.Mods` |
| `r2modman.Bases`, `gale.Bases` | Linux: the host's XDG home, never a Flatpak sandbox's own; Windows: `%AppData%`; elsewhere the user config directory |
| `r2modman.DataDirs`, `gale.DataDirs` | Only folders that exist, without repeats |

## Requirements

Go 1.27.3 or newer, plus `lefthook` and `gitleaks` on PATH if you want the git hooks.

```bash
lefthook install
```

That is per clone; git does not commit hooks. Pre-commit scans staged changes for secrets, and
pre-push runs `gate`.

## Verify

```bash
gate
```

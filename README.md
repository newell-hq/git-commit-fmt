# git-commit-fmt

A validating parser and pretty printer for raw git commit objects — the
bytes you get back from `git cat-file commit <sha>` (or from walking a
pack yourself), not `git log` output.

## Why

Git's commit object format looks simple until you actually have to parse
it: multi-line PGP signatures use a one-space continuation convention
lifted from RFC 822, merge commits have a variable number of parent
lines, octopus merges can carry repeated `mergetag` headers, and there's
no length prefix anywhere so a parser has to get the line-by-line state
machine exactly right. Most tools shell out to `git show` and scrape the
text output instead of dealing with any of that. This package parses the
object itself, rejects anything that doesn't match git's own format with
a line number attached, and can write the struct back out — either as
the canonical object bytes or as a short human-readable summary.

## Usage

```go
package main

import (
	"fmt"
	"os"

	commitfmt "github.com/newell-hq/git-commit-fmt"
)

func main() {
	// raw is what `git cat-file commit HEAD` prints, with the
	// "commit <size>\0" loose-object header already stripped.
	raw := []byte(
		"tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n" +
			"parent 7f1a3c9e8b2d4f60a1c5e9d3b7f2a4c6e8d0f1a2\n" +
			"author Jane Doe <jane@example.com> 1700000000 -0500\n" +
			"committer Jane Doe <jane@example.com> 1700000000 -0500\n" +
			"\n" +
			"Fix off-by-one in the changelog generator\n",
	)

	c, err := commitfmt.Parse(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid commit object:", err)
		os.Exit(1)
	}

	c.Pretty(os.Stdout)

	// c.Bytes() reproduces the canonical object form, so it round-trips
	// for anything git itself would have written.
	if string(c.Bytes()) != string(raw) {
		panic("did not round-trip")
	}
}
```

`Pretty` prints something close to `git log`'s default one-commit format:

```
tree      4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent    7f1a3c9e8b2d4f60a1c5e9d3b7f2a4c6e8d0f1a2
Author:   Jane Doe <jane@example.com>
Date:     Tue Nov 14 17:13:20 2023 -0500

    Fix off-by-one in the changelog generator
```

## What Parse validates

- header order: `tree`, then zero or more `parent` lines, then exactly
  one `author`, then exactly one `committer`, then anything else
  (`gpgsig`, `mergetag`, `encoding`, ...)
- `tree` and `parent` values are well-formed object ids (40 hex chars
  for sha1, 64 for sha256)
- `author`/`committer` lines match git's
  `Name <email> <unix-seconds> <±HHMM>` form
- a header/message separator (blank line) is actually present
- continuation lines (single leading space, used for multi-line values
  like a PGP signature block) always follow a real header

Errors are `*commitfmt.ParseError`, which carries the 1-based input line
number so a caller can point at the exact problem.

## What's not here yet

- no `PARSE_AND_OPEN` convenience for reading straight from `git
  cat-file`; callers currently supply the raw bytes themselves
- no charset conversion for the `encoding` header — the value is parsed
  and preserved, but the message bytes are passed through as-is
- no sha256-repo end-to-end test fixtures, only the object-id length
  check

## Status

Early skeleton: the parser and pretty printer both work and are
covered by a table-driven test suite in `commit_test.go`, but the API
should still be considered unstable.

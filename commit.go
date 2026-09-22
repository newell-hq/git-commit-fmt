// Package commitfmt parses and pretty-prints the raw contents of a git
// commit object, i.e. the bytes you get back from `git cat-file commit
// <sha>` (minus the "commit <size>\0" loose-object header, which callers
// are expected to have already stripped).
package commitfmt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Commit is the parsed form of a git commit object.
type Commit struct {
	Tree      string
	Parents   []string
	Author    Signature
	Committer Signature

	// Encoding is the value of the "encoding" header, if present. Git only
	// writes this when the commit message is not UTF-8.
	Encoding string

	// ExtraHeaders holds any header we don't otherwise model (gpgsig,
	// mergetag, and anything a future git adds), in the order they were
	// read. mergetag can legitimately repeat for octopus merges, so this
	// is a slice rather than a map.
	ExtraHeaders []Header

	// Message is the commit message with any trailing newline stripped
	// from the raw object (Bytes puts it back).
	Message string
}

// Header is a raw, unmodeled header line. Value has any continuation
// lines joined with "\n" and the leading continuation space removed.
type Header struct {
	Key   string
	Value string
}

// Signature is the parsed form of an "author" or "committer" line:
//
//	author Jane Doe <jane@example.com> 1700000000 -0500
type Signature struct {
	Name  string
	Email string

	// Seconds is the commit time as seconds since the Unix epoch.
	Seconds int64

	// Offset is the raw timezone offset, e.g. "-0500". It is kept as a
	// string rather than folded into a time.Location because git treats
	// "-0000" (unknown local offset) as distinct from "+0000" (UTC), and
	// that distinction is worth preserving rather than silently losing.
	Offset string
}

// ParseError reports a problem with a specific line of a commit object.
// Line numbers are 1-based and count raw lines in the input, including
// continuation lines.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func errAt(line int, format string, args ...any) error {
	return &ParseError{Line: line, Msg: fmt.Sprintf(format, args...)}
}

var (
	hexRE = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`) // sha1 or sha256
	sigRE = regexp.MustCompile(`^(.*) <(.*)> (\d+) ([+-]\d{4})$`)
)

// header-block stage, used to enforce git's fixed header ordering:
// tree, then parents, then author, then committer, then everything else.
const (
	stageTree = iota
	stageParent
	stageAuthor
	stageCommitter
)

// Parse validates and decodes the raw body of a git commit object. It
// returns a *ParseError for anything that doesn't match git's own format,
// so callers can report a precise line number.
func Parse(data []byte) (*Commit, error) {
	// A trailing "\n" is normal (git always writes one); strip it so the
	// final split doesn't produce a spurious extra empty line for
	// well-formed input.
	text := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(text, "\n")

	c := &Commit{}
	stage := stageTree
	sawEncoding := false
	sepLine := -1

	i := 0
	for ; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			sepLine = i
			break
		}
		if strings.HasPrefix(line, " ") {
			return nil, errAt(i+1, "continuation line with no preceding header")
		}

		key, value, ok := strings.Cut(line, " ")
		if !ok {
			return nil, errAt(i+1, "malformed header line %q", line)
		}
		startLine := i + 1

		// Fold in any continuation lines (git prefixes them with a
		// single literal space).
		for i+1 < len(lines) && strings.HasPrefix(lines[i+1], " ") {
			i++
			value += "\n" + lines[i][1:]
		}

		switch key {
		case "tree":
			if stage != stageTree {
				return nil, errAt(startLine, "tree header must be the first header")
			}
			if !hexRE.MatchString(value) {
				return nil, errAt(startLine, "tree is not a valid object id: %q", value)
			}
			c.Tree = value
			stage = stageParent

		case "parent":
			if stage != stageParent {
				return nil, errAt(startLine, "parent header must come after tree and before author")
			}
			if !hexRE.MatchString(value) {
				return nil, errAt(startLine, "parent is not a valid object id: %q", value)
			}
			c.Parents = append(c.Parents, value)
			stage = stageParent

		case "author":
			if stage != stageParent {
				return nil, errAt(startLine, "author header must come after tree and any parents")
			}
			sig, err := parseSignature(startLine, value)
			if err != nil {
				return nil, err
			}
			c.Author = sig
			stage = stageAuthor

		case "committer":
			if stage != stageAuthor {
				return nil, errAt(startLine, "committer header must directly follow author")
			}
			sig, err := parseSignature(startLine, value)
			if err != nil {
				return nil, err
			}
			c.Committer = sig
			stage = stageCommitter

		case "encoding":
			if stage < stageCommitter {
				return nil, errAt(startLine, "encoding header must come after committer")
			}
			if sawEncoding {
				return nil, errAt(startLine, "duplicate encoding header")
			}
			if value == "" {
				return nil, errAt(startLine, "encoding header has no value")
			}
			c.Encoding = value
			sawEncoding = true

		default:
			if stage < stageCommitter {
				return nil, errAt(startLine, "%q header must come after committer", key)
			}
			c.ExtraHeaders = append(c.ExtraHeaders, Header{Key: key, Value: value})
		}
	}

	if sepLine == -1 {
		return nil, errAt(len(lines), "missing blank line between headers and message")
	}
	if c.Tree == "" {
		return nil, errAt(1, "missing tree header")
	}
	if stage < stageCommitter {
		return nil, errAt(sepLine+1, "missing author or committer header")
	}

	c.Message = strings.Join(lines[sepLine+1:], "\n")

	return c, nil
}

func parseSignature(line int, value string) (Signature, error) {
	m := sigRE.FindStringSubmatch(value)
	if m == nil {
		return Signature{}, errAt(line, "malformed signature %q", value)
	}
	name, email, ts, tz := m[1], m[2], m[3], m[4]

	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Signature{}, errAt(line, "timestamp %q out of range", ts)
	}

	return Signature{
		Name:    name,
		Email:   email,
		Seconds: seconds,
		Offset:  tz,
	}, nil
}

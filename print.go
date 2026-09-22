package commitfmt

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// String renders a signature exactly as git would write it in a commit
// object: "Name <email> <seconds> <offset>".
func (s Signature) String() string {
	return s.Name + " <" + s.Email + "> " + strconv.FormatInt(s.Seconds, 10) + " " + s.Offset
}

// Time returns the signature's timestamp as a time.Time in a fixed zone
// built from Offset. "-0000" is treated the same as "+0000" here since
// time.FixedZone has no way to represent "unknown local offset" — use
// Offset directly if that distinction matters to you.
func (s Signature) Time() time.Time {
	sign := 1
	off := s.Offset
	if strings.HasPrefix(off, "-") {
		sign = -1
	}
	off = strings.TrimPrefix(strings.TrimPrefix(off, "+"), "-")
	hh, _ := strconv.Atoi(off[0:2])
	mm, _ := strconv.Atoi(off[2:4])
	secs := sign * (hh*3600 + mm*60)
	return time.Unix(s.Seconds, 0).In(time.FixedZone(s.Offset, secs))
}

// Bytes serializes the commit back into the canonical git object form.
// For a commit produced by Parse from well-formed git output, Bytes
// reproduces the original bytes exactly. It is only a normalization for
// inputs whose extra headers (gpgsig, mergetag, ...) were interleaved
// with "encoding" in a way real git never does, since Bytes always
// writes tree, parents, author, committer, encoding, then the remaining
// extra headers in their original relative order.
func (c *Commit) Bytes() []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "tree %s\n", c.Tree)
	for _, p := range c.Parents {
		fmt.Fprintf(&b, "parent %s\n", p)
	}
	fmt.Fprintf(&b, "author %s\n", c.Author)
	fmt.Fprintf(&b, "committer %s\n", c.Committer)
	if c.Encoding != "" {
		fmt.Fprintf(&b, "encoding %s\n", c.Encoding)
	}
	for _, h := range c.ExtraHeaders {
		fmt.Fprintf(&b, "%s %s\n", h.Key, fold(h.Value))
	}
	b.WriteByte('\n')
	b.WriteString(c.Message)
	b.WriteByte('\n')

	return []byte(b.String())
}

// fold re-adds the leading-space continuation prefix that Parse strips
// from multi-line header values.
func fold(value string) string {
	return strings.ReplaceAll(value, "\n", "\n ")
}

// Pretty writes a human-readable summary of the commit, in the style of
// `git log`'s default format.
func (c *Commit) Pretty(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "tree      %s\n", c.Tree); err != nil {
		return err
	}
	for _, p := range c.Parents {
		if _, err := fmt.Fprintf(w, "parent    %s\n", p); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Author:   %s <%s>\n", c.Author.Name, c.Author.Email); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Date:     %s\n", c.Author.Time().Format("Mon Jan 2 15:04:05 2006 -0700")); err != nil {
		return err
	}
	if c.Committer.Email != c.Author.Email || c.Committer.Seconds != c.Author.Seconds {
		if _, err := fmt.Fprintf(w, "Commit:   %s <%s>\n", c.Committer.Name, c.Committer.Email); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "CommitDate: %s\n", c.Committer.Time().Format("Mon Jan 2 15:04:05 2006 -0700")); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for _, line := range strings.Split(c.Message, "\n") {
		if line == "" {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "    %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

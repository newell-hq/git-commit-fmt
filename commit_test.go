package commitfmt

import (
	"bytes"
	"strings"
	"testing"
)

// hexHash builds a syntactically valid 40-char object id without anyone
// having to hand-count hex digits in a test fixture.
func hexHash(b byte) string { return strings.Repeat(string(b), 40) }

var (
	treeA = hexHash('a')
	treeB = hexHash('b')
	treeC = hexHash('c')

	author    = "Ada Lovelace <ada@example.com> 1700000000 -0500"
	committer = "Grace Hopper <grace@example.com> 1700000100 -0500"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // substring expected in the error; empty means it must parse cleanly
		check   func(t *testing.T, c *Commit)
	}{
		{
			name: "root commit with no parents",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"\n" +
				"Initial commit\n",
			check: func(t *testing.T, c *Commit) {
				if len(c.Parents) != 0 {
					t.Errorf("Parents = %v, want none", c.Parents)
				}
				if c.Message != "Initial commit" {
					t.Errorf("Message = %q", c.Message)
				}
			},
		},
		{
			name: "single parent",
			input: "tree " + treeA + "\n" +
				"parent " + treeB + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"\n" +
				"Fix a bug\n",
			check: func(t *testing.T, c *Commit) {
				if len(c.Parents) != 1 || c.Parents[0] != treeB {
					t.Errorf("Parents = %v", c.Parents)
				}
			},
		},
		{
			name: "merge commit with two parents",
			input: "tree " + treeA + "\n" +
				"parent " + treeB + "\n" +
				"parent " + treeC + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"\n" +
				"Merge branch 'x'\n",
			check: func(t *testing.T, c *Commit) {
				if len(c.Parents) != 2 {
					t.Fatalf("Parents = %v, want 2", c.Parents)
				}
				if c.Parents[0] != treeB || c.Parents[1] != treeC {
					t.Errorf("Parents = %v, order not preserved", c.Parents)
				}
			},
		},
		{
			name: "empty message",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"\n",
			check: func(t *testing.T, c *Commit) {
				if c.Message != "" {
					t.Errorf("Message = %q, want empty", c.Message)
				}
			},
		},
		{
			name: "message with internal blank lines is preserved verbatim",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"\n" +
				"Subject\n\nBody line one\n\nBody line two\n",
			check: func(t *testing.T, c *Commit) {
				want := "Subject\n\nBody line one\n\nBody line two"
				if c.Message != want {
					t.Errorf("Message = %q, want %q", c.Message, want)
				}
			},
		},
		{
			name: "encoding header",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"encoding ISO-8859-1\n" +
				"\n" +
				"commit avec des accents\n",
			check: func(t *testing.T, c *Commit) {
				if c.Encoding != "ISO-8859-1" {
					t.Errorf("Encoding = %q", c.Encoding)
				}
			},
		},
		{
			name: "gpgsig continuation lines are folded and unfolded correctly",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"gpgsig -----BEGIN PGP SIGNATURE-----\n" +
				" \n" +
				" iQEzBAABCAAdFiEE1234567890abcdefghijklmnopqrstuv\n" +
				" =AbCd\n" +
				" -----END PGP SIGNATURE-----\n" +
				"\n" +
				"Signed commit\n",
			check: func(t *testing.T, c *Commit) {
				if len(c.ExtraHeaders) != 1 || c.ExtraHeaders[0].Key != "gpgsig" {
					t.Fatalf("ExtraHeaders = %+v", c.ExtraHeaders)
				}
				want := "-----BEGIN PGP SIGNATURE-----\n" +
					"\n" +
					"iQEzBAABCAAdFiEE1234567890abcdefghijklmnopqrstuv\n" +
					"=AbCd\n" +
					"-----END PGP SIGNATURE-----"
				if c.ExtraHeaders[0].Value != want {
					t.Errorf("gpgsig value = %q, want %q", c.ExtraHeaders[0].Value, want)
				}
			},
		},
		{
			name: "repeated mergetag headers from an octopus merge",
			input: "tree " + treeA + "\n" +
				"parent " + treeB + "\n" +
				"parent " + treeC + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"mergetag object " + treeB + "\n" +
				" type commit\n" +
				" tag v1\n" +
				"mergetag object " + treeC + "\n" +
				" type commit\n" +
				" tag v2\n" +
				"\n" +
				"Merge tags 'v1' and 'v2'\n",
			check: func(t *testing.T, c *Commit) {
				if len(c.ExtraHeaders) != 2 {
					t.Fatalf("ExtraHeaders = %+v, want 2 mergetag entries", c.ExtraHeaders)
				}
			},
		},
		{
			name:    "missing tree header",
			input:   "author " + author + "\n" + "committer " + committer + "\n\nmsg\n",
			wantErr: "author header must come after tree",
		},
		{
			name: "duplicate tree header",
			input: "tree " + treeA + "\n" +
				"tree " + treeB + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "tree header must be the first header",
		},
		{
			name:    "invalid tree object id",
			input:   "tree not-a-valid-hash\n" + "author " + author + "\n" + "committer " + committer + "\n\nmsg\n",
			wantErr: "not a valid object id",
		},
		{
			name: "parent before tree",
			input: "parent " + treeB + "\n" +
				"tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "parent header must come after tree",
		},
		{
			name: "committer without a preceding author",
			input: "tree " + treeA + "\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "committer header must directly follow author",
		},
		{
			name: "extra header sandwiched between author and committer",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"gpgsig fake\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: `"gpgsig" header must come after committer`,
		},
		{
			name: "author line missing the email",
			input: "tree " + treeA + "\n" +
				"author Ada Lovelace 1700000000 -0500\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "malformed signature",
		},
		{
			name: "author line with a non-numeric timestamp",
			input: "tree " + treeA + "\n" +
				"author Ada Lovelace <ada@example.com> not-a-timestamp -0500\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "malformed signature",
		},
		{
			name: "author line with a malformed timezone offset",
			input: "tree " + treeA + "\n" +
				"author Ada Lovelace <ada@example.com> 1700000000 +5\n" +
				"committer " + committer + "\n\nmsg\n",
			wantErr: "malformed signature",
		},
		{
			name: "missing blank line between headers and message",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"extra-header still no blank line after this\n",
			wantErr: "missing blank line between headers and message",
		},
		{
			name:    "continuation line with no preceding header",
			input:   " stray continuation\ntree " + treeA + "\n\nmsg\n",
			wantErr: "continuation line with no preceding header",
		},
		{
			name: "duplicate encoding header",
			input: "tree " + treeA + "\n" +
				"author " + author + "\n" +
				"committer " + committer + "\n" +
				"encoding UTF-16\n" +
				"encoding UTF-8\n\nmsg\n",
			wantErr: "duplicate encoding header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Parse([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse succeeded, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if tt.check != nil {
				tt.check(t, c)
			}
		})
	}
}

// TestRoundTrip checks that well-formed input, once parsed, comes back
// out of Bytes byte-for-byte: real git always writes headers in this
// exact order, so this is the case that actually matters.
func TestRoundTrip(t *testing.T) {
	inputs := []string{
		"tree " + treeA + "\n" +
			"author " + author + "\n" +
			"committer " + committer + "\n" +
			"\n" +
			"Initial commit\n",
		"tree " + treeA + "\n" +
			"parent " + treeB + "\n" +
			"parent " + treeC + "\n" +
			"author " + author + "\n" +
			"committer " + committer + "\n" +
			"encoding ISO-8859-1\n" +
			"\n" +
			"Merge branch 'x'\n\nwith a body\n",
		"tree " + treeA + "\n" +
			"author " + author + "\n" +
			"committer " + committer + "\n" +
			"gpgsig -----BEGIN PGP SIGNATURE-----\n" +
			" \n" +
			" iQEzBAABCAAdFiEE1234567890abcdefghijklmnopqrstuv\n" +
			" -----END PGP SIGNATURE-----\n" +
			"\n" +
			"Signed commit\n",
	}

	for _, in := range inputs {
		c, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		got := c.Bytes()
		if !bytes.Equal(got, []byte(in)) {
			t.Errorf("round trip mismatch:\n got: %q\nwant: %q", got, in)
		}
	}
}

func TestPretty(t *testing.T) {
	c, err := Parse([]byte("tree " + treeA + "\n" +
		"parent " + treeB + "\n" +
		"author " + author + "\n" +
		"committer " + committer + "\n" +
		"\n" +
		"Subject line\n\nBody.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var buf bytes.Buffer
	if err := c.Pretty(&buf); err != nil {
		t.Fatalf("Pretty failed: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Author:   Ada Lovelace <ada@example.com>",
		"Commit:   Grace Hopper <grace@example.com>",
		"Subject line",
		"Body.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Pretty output missing %q, got:\n%s", want, out)
		}
	}
}

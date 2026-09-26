package commitfmt

import "testing"

func mustParse(t *testing.T, input string) *Commit {
	t.Helper()
	c, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return c
}

func TestDiffIdentical(t *testing.T) {
	input := "tree " + treeA + "\n" +
		"author " + author + "\n" +
		"committer " + committer + "\n" +
		"\n" +
		"Initial commit\n"

	a := mustParse(t, input)
	b := mustParse(t, input)

	if got := Diff(a, b); got != "" {
		t.Errorf("Diff of identical commits = %q, want empty", got)
	}
}

func TestDiffMessageChange(t *testing.T) {
	a := mustParse(t, "tree "+treeA+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Fix a bug\n")
	b := mustParse(t, "tree "+treeA+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Fix a different bug\n")

	got := Diff(a, b)
	want := "@@ -2,4 +2,4 @@\n" +
		" author " + author + "\n" +
		" committer " + committer + "\n" +
		" \n" +
		"-Fix a bug\n" +
		"+Fix a different bug\n"
	if got != want {
		t.Errorf("Diff:\n got: %q\nwant: %q", got, want)
	}
}

func TestDiffParentAdded(t *testing.T) {
	a := mustParse(t, "tree "+treeA+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Merge\n")
	b := mustParse(t, "tree "+treeA+"\n"+
		"parent "+treeB+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Merge\n")

	got := Diff(a, b)
	want := "@@ -1,4 +1,5 @@\n" +
		" tree " + treeA + "\n" +
		"+parent " + treeB + "\n" +
		" author " + author + "\n" +
		" committer " + committer + "\n" +
		" \n"
	if got != want {
		t.Errorf("Diff:\n got: %q\nwant: %q", got, want)
	}
}

// TestDiffFarApartChanges checks that two changes separated by enough
// unchanged lines produce two separate hunks rather than one giant one.
func TestDiffFarApartChanges(t *testing.T) {
	a := mustParse(t, "tree "+treeA+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Subject\n\none\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	b := mustParse(t, "tree "+treeA+"\n"+
		"author "+author+"\n"+
		"committer "+committer+"\n"+
		"\n"+
		"Changed subject\n\none\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten changed\n")

	got := Diff(a, b)
	if n := countOccurrences(got, "@@"); n != 4 {
		t.Errorf("got %d hunk markers (2 hunks), want 4:\n%s", n, got)
	}
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

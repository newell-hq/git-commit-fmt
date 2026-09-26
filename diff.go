package commitfmt

import (
	"fmt"
	"strings"
)

// contextLines is the number of unchanged lines kept around each change
// in a hunk, matching the default of `diff -u` and `git diff`.
const contextLines = 3

// Diff returns a unified diff between the canonical byte forms of a and
// b, in the style of `diff -u`. It's meant for comparing two versions of
// what is conceptually "the same" commit — before and after a rebase,
// an amend, a re-sign — not for showing what changed in the tree, since
// this package never reads tree or blob objects and has no way to
// produce a file-level patch.
//
// Diff returns "" if a and b serialize identically.
func Diff(a, b *Commit) string {
	return formatUnified(diffLines(splitLines(a.Bytes()), splitLines(b.Bytes())))
}

func splitLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// diffOp is one line of an edit script turning a into b.
type diffOp struct {
	kind byte // ' ' (unchanged), '-' (only in a), or '+' (only in b)
	text string
}

// diffLines computes a minimal edit script from a to b using the
// standard LCS dynamic-programming recurrence. Commit objects are at
// most a few dozen lines, so the O(len(a)*len(b)) table is cheap.
func diffLines(a, b []string) []diffOp {
	n, m := len(a), len(b)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

// hunkSpan is a half-open range of op indices, [start, end), that forms
// one unified-diff hunk.
type hunkSpan struct{ start, end int }

// formatUnified groups an edit script into unified-diff hunks (each
// change plus contextLines of surrounding unchanged lines, with
// adjacent hunks merged when their context would overlap) and renders
// them with "@@ -aStart,aCount +bStart,bCount @@" headers.
func formatUnified(ops []diffOp) string {
	var changes []hunkSpan
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := i
		for i < len(ops) && ops[i].kind != ' ' {
			i++
		}
		changes = append(changes, hunkSpan{start, i})
	}
	if len(changes) == 0 {
		return ""
	}

	var hunks []hunkSpan
	for _, c := range changes {
		lo := c.start - contextLines
		if lo < 0 {
			lo = 0
		}
		hi := c.end + contextLines
		if hi > len(ops) {
			hi = len(ops)
		}
		if n := len(hunks); n > 0 && lo <= hunks[n-1].end {
			hunks[n-1].end = hi
		} else {
			hunks = append(hunks, hunkSpan{lo, hi})
		}
	}

	// aLine[i]/bLine[i] is the 1-based line number in a/b that op i
	// would occupy, i.e. the running position just before op i runs.
	aLine := make([]int, len(ops)+1)
	bLine := make([]int, len(ops)+1)
	aLine[0], bLine[0] = 1, 1
	for idx, op := range ops {
		aLine[idx+1], bLine[idx+1] = aLine[idx], bLine[idx]
		if op.kind != '+' {
			aLine[idx+1]++
		}
		if op.kind != '-' {
			bLine[idx+1]++
		}
	}

	var out strings.Builder
	for _, h := range hunks {
		var aCount, bCount int
		for _, op := range ops[h.start:h.end] {
			if op.kind != '+' {
				aCount++
			}
			if op.kind != '-' {
				bCount++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", aLine[h.start], aCount, bLine[h.start], bCount)
		for _, op := range ops[h.start:h.end] {
			fmt.Fprintf(&out, "%c%s\n", op.kind, op.text)
		}
	}
	return out.String()
}

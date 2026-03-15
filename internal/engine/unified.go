package engine

import (
	"fmt"
	"strings"
)

// GenerateUnifiedDiff produces a unified diff (full context) between old and
// new text content, compatible with the output of
// PhabricatorDifferenceEngine::generateRawDiffFromFileContent.
func GenerateUnifiedDiff(req *DiffRequest) *DiffResult {
	oldName := req.OldName
	if oldName == "" {
		oldName = "/dev/universe"
	}
	newName := req.NewName
	if newName == "" {
		newName = "/dev/universe"
	}

	oldText := req.Old
	newText := req.New

	if req.Normalize {
		oldText = normalizeText(oldText)
		newText = normalizeText(newText)
	}

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	if linesEqual(oldLines, newLines) {
		return &DiffResult{
			Diff:  buildIdenticalDiff(oldName, newName, oldLines),
			Equal: true,
		}
	}

	ops := lcs(oldLines, newLines)
	diff := formatUnified(oldName, newName, oldLines, newLines, ops)

	return &DiffResult{
		Diff:  diff,
		Equal: false,
	}
}

func normalizeText(s string) string {
	return strings.NewReplacer(" ", "", "\t", "").Replace(s)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildIdenticalDiff(oldName, newName string, lines []string) string {
	n := len(lines)
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s 9999-99-99\n", oldName)
	fmt.Fprintf(&b, "+++ %s 9999-99-99\n", newName)

	if n == 0 {
		return b.String()
	}

	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", n, n)
	for i, line := range lines {
		b.WriteByte(' ')
		b.WriteString(line)
		if i < n-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')

	return b.String()
}

// editOp represents an operation in the edit script.
type editOp byte

const (
	opEqual  editOp = '='
	opDelete editOp = '-'
	opInsert editOp = '+'
)

// lcs computes a diff using an LCS-based (longest common subsequence) DP
// approach. This is O(n*m) but simple and correct.
func lcs(a, b []string) []editOp {
	n := len(a)
	m := len(b)

	if n == 0 {
		ops := make([]editOp, m)
		for i := range ops {
			ops[i] = opInsert
		}
		return ops
	}
	if m == 0 {
		ops := make([]editOp, n)
		for i := range ops {
			ops[i] = opDelete
		}
		return ops
	}

	// DP table
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce edit operations
	var ops []editOp
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append(ops, opEqual)
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, opInsert)
			j--
		} else {
			ops = append(ops, opDelete)
			i--
		}
	}

	// Reverse
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}

	return ops
}

func formatUnified(oldName, newName string, oldLines, newLines []string, ops []editOp) string {
	type lineRec struct {
		op   editOp
		text string
	}

	var recs []lineRec
	oi, ni := 0, 0
	for _, op := range ops {
		switch op {
		case opEqual:
			recs = append(recs, lineRec{op: opEqual, text: oldLines[oi]})
			oi++
			ni++
		case opDelete:
			recs = append(recs, lineRec{op: opDelete, text: oldLines[oi]})
			oi++
		case opInsert:
			recs = append(recs, lineRec{op: opInsert, text: newLines[ni]})
			ni++
		}
	}

	oldCount := 0
	newCount := 0
	for _, r := range recs {
		switch r.op {
		case opEqual:
			oldCount++
			newCount++
		case opDelete:
			oldCount++
		case opInsert:
			newCount++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s 9999-99-99\n", oldName)
	fmt.Fprintf(&b, "+++ %s 9999-99-99\n", newName)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", oldCount, newCount)
	for i, r := range recs {
		switch r.op {
		case opEqual:
			b.WriteByte(' ')
		case opDelete:
			b.WriteByte('-')
		case opInsert:
			b.WriteByte('+')
		}
		b.WriteString(r.text)
		if i < len(recs)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')

	return b.String()
}

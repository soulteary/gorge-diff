package engine

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// GenerateProseDiff produces a multi-level prose diff compatible with
// PhutilProseDifferenceEngine::getDiff.  It progresses through four
// granularity levels (paragraphs → sentences → words → characters) to
// produce a human-friendly result.
func GenerateProseDiff(req *ProseRequest) *ProseResult {
	parts := buildProseDiff(req.Old, req.New, 0)
	parts = reorderParts(parts)
	return &ProseResult{Parts: parts}
}

func buildProseDiff(u, v string, level int) []ProsePart {
	uParts := splitCorpus(u, level)
	vParts := splitCorpus(v, level)

	var diff []ProsePart
	var tooLarge bool

	if level == 0 {
		diff = hashDiff(uParts, vParts)
	} else {
		diff, tooLarge = editDistanceDiff(uParts, vParts, level)
	}

	diff = reorderParts(diff)

	if level == 3 {
		return diff
	}

	// Group consecutive -/+ into change blocks and recurse at finer granularity.
	type block struct {
		kind string // "=" or "!"
		text string
		old  string
		new_ string
	}

	var blocks []block
	var cur *block

	for _, p := range diff {
		switch p.Type {
		case "=":
			if cur != nil {
				blocks = append(blocks, *cur)
				cur = nil
			}
			blocks = append(blocks, block{kind: "=", text: p.Text})
		case "-":
			if cur == nil {
				cur = &block{kind: "!"}
			}
			cur.old += p.Text
		case "+":
			if cur == nil {
				cur = &block{kind: "!"}
			}
			cur.new_ += p.Text
		}
	}
	if cur != nil {
		blocks = append(blocks, *cur)
	}

	var result []ProsePart
	for _, blk := range blocks {
		if blk.kind == "=" {
			result = append(result, ProsePart{Type: "=", Text: blk.text})
			continue
		}

		old := blk.old
		new_ := blk.new_

		if old == "" && new_ == "" {
			continue
		} else if old == "" {
			result = append(result, ProsePart{Type: "+", Text: new_})
		} else if new_ == "" {
			result = append(result, ProsePart{Type: "-", Text: old})
		} else if tooLarge {
			result = append(result, ProsePart{Type: "-", Text: old})
			result = append(result, ProsePart{Type: "+", Text: new_})
		} else {
			sub := buildProseDiff(old, new_, level+1)
			result = append(result, sub...)
		}
	}

	result = reorderParts(result)
	return result
}

var (
	reParagraph = regexp.MustCompile(`(\n+)`)
	reSentence  = regexp.MustCompile(`([\n,!;?.]+)`)
	reWord      = regexp.MustCompile(`(\s+)`)
)

func splitCorpus(corpus string, level int) []string {
	switch level {
	case 0:
		return stitchPieces(reParagraph.Split(corpus, -1), reParagraph.FindAllString(corpus, -1), level)
	case 1:
		return stitchPieces(reSentence.Split(corpus, -1), reSentence.FindAllString(corpus, -1), level)
	case 2:
		return stitchPieces(reWord.Split(corpus, -1), reWord.FindAllString(corpus, -1), level)
	case 3:
		return splitChars(corpus)
	}
	return []string{corpus}
}

func splitChars(s string) []string {
	var result []string
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		result = append(result, string(r))
		s = s[size:]
	}
	return result
}

func stitchPieces(parts []string, delims []string, level int) []string {
	var results []string
	for i, part := range parts {
		piece := part
		if i < len(delims) {
			piece += delims[i]
		}

		if level < 2 {
			trimmed := trimApart(piece)
			results = append(results, trimmed...)
		} else {
			results = append(results, piece)
		}
	}

	if len(results) > 0 && results[len(results)-1] == "" {
		results = results[:len(results)-1]
	}

	return results
}

// trimApart splits a string into [leading-whitespace, body, trailing-whitespace].
func trimApart(input string) []string {
	if input == "" {
		return nil
	}

	var parts []string
	corpus := strings.TrimLeft(input, " \t\n\r")
	if len(corpus) != len(input) {
		parts = append(parts, input[:len(input)-len(corpus)])
	}

	trimmed := strings.TrimRight(corpus, " \t\n\r")
	if len(trimmed) > 0 {
		parts = append(parts, trimmed)
	}

	if len(trimmed) != len(corpus) {
		parts = append(parts, corpus[len(trimmed):])
	}

	return parts
}

// hashDiff aligns blocks by content hash, matching the paragraph-level
// alignment in PhutilProseDifferenceEngine::newHashDiff.
func hashDiff(uParts, vParts []string) []ProsePart {
	vMap := make(map[string][]int)
	for i, p := range vParts {
		vMap[p] = append(vMap[p], i)
	}

	vUsed := make([]bool, len(vParts))
	// Track which v index is matched for each u
	matches := make([]int, len(uParts))
	for i := range matches {
		matches[i] = -1
	}

	// Greedy LCS-style forward matching
	vNext := 0
	for i, up := range uParts {
		indices := vMap[up]
		for _, vi := range indices {
			if vi >= vNext && !vUsed[vi] {
				matches[i] = vi
				vUsed[vi] = true
				vNext = vi + 1
				break
			}
		}
	}

	// Build diff from matches
	var parts []ProsePart
	vi := 0
	for ui := 0; ui < len(uParts); ui++ {
		mi := matches[ui]
		if mi < 0 {
			parts = append(parts, ProsePart{Type: "-", Text: uParts[ui]})
			continue
		}
		// Emit unmatched v parts before this match
		for vi < mi {
			if !isVMatched(vi, matches) {
				parts = append(parts, ProsePart{Type: "+", Text: vParts[vi]})
			}
			vi++
		}
		parts = append(parts, ProsePart{Type: "=", Text: uParts[ui]})
		vi = mi + 1
	}

	// Remaining v parts
	for vi < len(vParts) {
		if !vUsed[vi] {
			parts = append(parts, ProsePart{Type: "+", Text: vParts[vi]})
		}
		vi++
	}

	return parts
}

func isVMatched(vi int, matches []int) bool {
	for _, m := range matches {
		if m == vi {
			return true
		}
	}
	return false
}

const maxEditDistance = 128

// editDistanceDiff computes diff using edit distance, matching
// PhutilProseDifferenceEngine::newEditDistanceMatrixDiff.
func editDistanceDiff(uParts, vParts []string, level int) ([]ProsePart, bool) {
	n := len(uParts)
	m := len(vParts)

	tooLarge := n > maxEditDistance || m > maxEditDistance
	if tooLarge {
		// Fall back to simple delete-old/insert-new
		var parts []ProsePart
		for _, p := range uParts {
			parts = append(parts, ProsePart{Type: "-", Text: p})
		}
		for _, p := range vParts {
			parts = append(parts, ProsePart{Type: "+", Text: p})
		}
		return parts, true
	}

	// Standard edit distance DP
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if uParts[i-1] == vParts[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				sub := dp[i-1][j-1] + 1
				del := dp[i-1][j] + 1
				ins := dp[i][j-1] + 1
				dp[i][j] = min3(sub, del, ins)
			}
		}
	}

	// Backtrack to build edit string
	var ops []byte
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && uParts[i-1] == vParts[j-1] {
			ops = append(ops, 's')
			i--
			j--
		} else if i > 0 && j > 0 && dp[i][j] == dp[i-1][j-1]+1 {
			ops = append(ops, 'x')
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] <= dp[i-1][j]) {
			ops = append(ops, 'i')
			j--
		} else {
			ops = append(ops, 'd')
			i--
		}
	}

	// Reverse
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}

	// Apply smoothing for word-level and character-level diffs
	if level > 1 {
		ops = smooth(ops)
	}

	var parts []ProsePart
	ui, vi := 0, 0
	for _, c := range ops {
		switch c {
		case 's':
			parts = append(parts, ProsePart{Type: "=", Text: uParts[ui]})
			ui++
			vi++
		case 'd':
			parts = append(parts, ProsePart{Type: "-", Text: uParts[ui]})
			ui++
		case 'i':
			parts = append(parts, ProsePart{Type: "+", Text: vParts[vi]})
			vi++
		case 'x':
			parts = append(parts, ProsePart{Type: "-", Text: uParts[ui]})
			parts = append(parts, ProsePart{Type: "+", Text: vParts[vi]})
			ui++
			vi++
		}
	}

	return parts, false
}

// smooth converts isolated 's' (same) ops surrounded by changes into 'x'
// (substitute), producing a less choppy output.
func smooth(ops []byte) []byte {
	result := make([]byte, len(ops))
	copy(result, ops)

	for i := range result {
		if result[i] != 's' {
			continue
		}
		prevChange := i > 0 && result[i-1] != 's'
		nextChange := i < len(result)-1 && result[i+1] != 's'
		if prevChange && nextChange {
			result[i] = 'x'
		}
	}
	return result
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// reorderParts groups consecutive -/+ parts so all deletions come before
// insertions, then merges adjacent parts of the same type.
// Matches PhutilProseDiff::reorderParts.
func reorderParts(parts []ProsePart) []ProsePart {
	var oRun, nRun []ProsePart
	var result []ProsePart

	flush := func() {
		if len(oRun) > 0 || len(nRun) > 0 {
			result = append(result, combineRuns(oRun, nRun)...)
			oRun = nil
			nRun = nil
		}
	}

	for _, p := range parts {
		switch p.Type {
		case "-":
			oRun = append(oRun, p)
		case "+":
			nRun = append(nRun, p)
		default:
			flush()
			result = append(result, p)
		}
	}
	flush()

	// Merge consecutive parts of the same type
	var combined []ProsePart
	for _, p := range result {
		if len(combined) > 0 && combined[len(combined)-1].Type == p.Type {
			combined[len(combined)-1].Text += p.Text
		} else {
			combined = append(combined, p)
		}
	}

	return combined
}

var layoutChars = map[byte]bool{
	' ': true, '\n': true, '.': true, '!': true, ',': true,
	'?': true, ']': true, '[': true, '(': true, ')': true,
	'<': true, '>': true,
}

// combineRuns merges delete and insert runs, extracting common
// layout-character prefixes/suffixes as unchanged spans.
func combineRuns(oRun, nRun []ProsePart) []ProsePart {
	oText := mergeRunText(oRun)
	nText := mergeRunText(nRun)

	oLen := len(oText)
	nLen := len(nText)
	minLen := oLen
	if nLen < minLen {
		minLen = nLen
	}

	prefixLen := 0
	for i := 0; i < minLen; i++ {
		if oText[i] != nText[i] || !layoutChars[oText[i]] {
			break
		}
		prefixLen++
	}

	suffixLen := 0
	for i := 0; i < minLen-prefixLen; i++ {
		o := oText[oLen-1-i]
		n := nText[nLen-1-i]
		if o != n || !layoutChars[o] {
			break
		}
		suffixLen++
	}

	var result []ProsePart

	if prefixLen > 0 {
		result = append(result, ProsePart{Type: "=", Text: oText[:prefixLen]})
	}
	if prefixLen < oLen {
		result = append(result, ProsePart{Type: "-", Text: oText[prefixLen : oLen-suffixLen]})
	}
	if prefixLen < nLen {
		result = append(result, ProsePart{Type: "+", Text: nText[prefixLen : nLen-suffixLen]})
	}
	if suffixLen > 0 {
		result = append(result, ProsePart{Type: "=", Text: oText[oLen-suffixLen:]})
	}

	return result
}

func mergeRunText(parts []ProsePart) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

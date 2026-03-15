package engine

import (
	"strings"
	"testing"
)

func TestGenerateUnifiedDiff_Identical(t *testing.T) {
	req := &DiffRequest{
		Old:     "line1\nline2\nline3\n",
		New:     "line1\nline2\nline3\n",
		OldName: "a.txt",
		NewName: "b.txt",
	}
	result := GenerateUnifiedDiff(req)

	if !result.Equal {
		t.Fatal("expected equal=true")
	}
	if !strings.Contains(result.Diff, "--- a.txt 9999-99-99") {
		t.Fatalf("missing old header: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+++ b.txt 9999-99-99") {
		t.Fatalf("missing new header: %s", result.Diff)
	}
	if strings.ContainsAny(result.Diff[strings.Index(result.Diff, "@@")+4:], "-+") {
		// Should only contain context lines (space prefix), no -/+
		// But we need to be careful: the text "9999-99-99" has dashes.
		// Check that hunk body lines start with space.
		lines := strings.Split(result.Diff, "\n")
		for _, line := range lines[3:] { // skip headers and hunk marker
			if line == "" {
				continue
			}
			if line[0] != ' ' {
				t.Fatalf("expected context line, got: %q", line)
			}
		}
	}
}

func TestGenerateUnifiedDiff_Changed(t *testing.T) {
	req := &DiffRequest{
		Old: "hello\nworld\n",
		New: "hello\ngopher\n",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "-world") {
		t.Fatalf("expected -world in diff: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+gopher") {
		t.Fatalf("expected +gopher in diff: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_Addition(t *testing.T) {
	req := &DiffRequest{
		Old: "a\nb\n",
		New: "a\nb\nc\n",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "+c") {
		t.Fatalf("expected +c in diff: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_Deletion(t *testing.T) {
	req := &DiffRequest{
		Old: "a\nb\nc\n",
		New: "a\nc\n",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "-b") {
		t.Fatalf("expected -b in diff: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_Empty(t *testing.T) {
	req := &DiffRequest{Old: "", New: ""}
	result := GenerateUnifiedDiff(req)

	if !result.Equal {
		t.Fatal("expected equal=true for empty inputs")
	}
}

func TestGenerateUnifiedDiff_DefaultNames(t *testing.T) {
	req := &DiffRequest{
		Old: "x\n",
		New: "y\n",
	}
	result := GenerateUnifiedDiff(req)

	if !strings.Contains(result.Diff, "--- /dev/universe 9999-99-99") {
		t.Fatalf("expected default old name: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+++ /dev/universe 9999-99-99") {
		t.Fatalf("expected default new name: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_Normalize(t *testing.T) {
	req := &DiffRequest{
		Old:       "hello world\n",
		New:       "helloworld\n",
		Normalize: true,
	}
	result := GenerateUnifiedDiff(req)

	if !result.Equal {
		t.Fatalf("expected equal=true after normalization, diff: %s", result.Diff)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a\n", 1},
		{"a\nb\n", 2},
		{"a\nb", 2},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.expected {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(got), tt.expected)
		}
	}
}

func TestLCS_AllInsert(t *testing.T) {
	ops := lcs(nil, []string{"a", "b"})
	for _, op := range ops {
		if op != opInsert {
			t.Fatalf("expected all insert ops, got %c", op)
		}
	}
}

func TestLCS_AllDelete(t *testing.T) {
	ops := lcs([]string{"a", "b"}, nil)
	for _, op := range ops {
		if op != opDelete {
			t.Fatalf("expected all delete ops, got %c", op)
		}
	}
}

func TestLCS_Mixed(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"a", "x", "c", "y"}
	ops := lcs(a, b)

	hasEq, hasDel, hasIns := false, false, false
	for _, op := range ops {
		switch op {
		case opEqual:
			hasEq = true
		case opDelete:
			hasDel = true
		case opInsert:
			hasIns = true
		}
	}
	if !hasEq || !hasDel || !hasIns {
		t.Fatalf("expected mix of equal/delete/insert, ops=%v", ops)
	}

	eqCount := 0
	for _, op := range ops {
		if op == opEqual {
			eqCount++
		}
	}
	if eqCount != 2 {
		t.Fatalf("expected 2 equal ops (a,c), got %d", eqCount)
	}
}

func TestLCS_CompleteReplace(t *testing.T) {
	a := []string{"x", "y"}
	b := []string{"m", "n"}
	ops := lcs(a, b)

	for _, op := range ops {
		if op == opEqual {
			t.Fatal("expected no equal ops for complete replacement")
		}
	}
}

func TestLCS_BothEmpty(t *testing.T) {
	ops := lcs(nil, nil)
	if len(ops) != 0 {
		t.Fatalf("expected empty ops for both empty inputs, got %d", len(ops))
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "helloworld"},
		{"hello\tworld", "helloworld"},
		{"  a  b  ", "ab"},
		{"nochange", "nochange"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeText(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeText(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestLinesEqual(t *testing.T) {
	tests := []struct {
		a, b     []string
		expected bool
	}{
		{nil, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a", "b"}, []string{"a"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
	}
	for _, tt := range tests {
		got := linesEqual(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("linesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestBuildIdenticalDiff_SingleLine(t *testing.T) {
	diff := buildIdenticalDiff("old.txt", "new.txt", []string{"only"})
	if !strings.Contains(diff, "--- old.txt 9999-99-99") {
		t.Fatalf("missing old header: %s", diff)
	}
	if !strings.Contains(diff, "@@ -1,1 +1,1 @@") {
		t.Fatalf("bad hunk header: %s", diff)
	}
	if !strings.Contains(diff, " only") {
		t.Fatalf("missing context line: %s", diff)
	}
}

func TestBuildIdenticalDiff_Empty(t *testing.T) {
	diff := buildIdenticalDiff("a", "b", nil)
	if strings.Contains(diff, "@@") {
		t.Fatalf("empty diff should not have hunk header: %s", diff)
	}
	if !strings.Contains(diff, "--- a 9999-99-99") {
		t.Fatalf("missing old header: %s", diff)
	}
}

func TestFormatUnified_InsertOnly(t *testing.T) {
	ops := []editOp{opInsert, opInsert}
	result := formatUnified("a", "b", nil, []string{"x", "y"}, ops)
	if !strings.Contains(result, "+x") || !strings.Contains(result, "+y") {
		t.Fatalf("expected insert lines: %s", result)
	}
	if !strings.Contains(result, "@@ -1,0 +1,2 @@") {
		t.Fatalf("bad hunk header: %s", result)
	}
}

func TestFormatUnified_DeleteOnly(t *testing.T) {
	ops := []editOp{opDelete, opDelete}
	result := formatUnified("a", "b", []string{"x", "y"}, nil, ops)
	if !strings.Contains(result, "-x") || !strings.Contains(result, "-y") {
		t.Fatalf("expected delete lines: %s", result)
	}
	if !strings.Contains(result, "@@ -1,2 +1,0 @@") {
		t.Fatalf("bad hunk header: %s", result)
	}
}

func TestGenerateUnifiedDiff_MultiLineChange(t *testing.T) {
	req := &DiffRequest{
		Old:     "line1\nline2\nline3\nline4\n",
		New:     "line1\nchanged2\nline3\nchanged4\n",
		OldName: "test.go",
		NewName: "test.go",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "-line2") {
		t.Fatalf("expected -line2: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+changed2") {
		t.Fatalf("expected +changed2: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "-line4") {
		t.Fatalf("expected -line4: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_OnlyNewContent(t *testing.T) {
	req := &DiffRequest{
		Old: "",
		New: "new content\n",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "+new content") {
		t.Fatalf("expected +new content: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_OnlyOldContent(t *testing.T) {
	req := &DiffRequest{
		Old: "old content\n",
		New: "",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "-old content") {
		t.Fatalf("expected -old content: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_NormalizeWithTabs(t *testing.T) {
	req := &DiffRequest{
		Old:       "hello\tworld\n",
		New:       "helloworld\n",
		Normalize: true,
	}
	result := GenerateUnifiedDiff(req)

	if !result.Equal {
		t.Fatalf("expected equal=true after tab normalization, diff: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_NormalizeDifferent(t *testing.T) {
	req := &DiffRequest{
		Old:       "hello world\n",
		New:       "helloxworld\n",
		Normalize: true,
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false: content differs even after normalization")
	}
}

func TestGenerateUnifiedDiff_SingleLine(t *testing.T) {
	req := &DiffRequest{
		Old: "only\n",
		New: "only\n",
	}
	result := GenerateUnifiedDiff(req)

	if !result.Equal {
		t.Fatal("expected equal=true")
	}
	if !strings.Contains(result.Diff, " only") {
		t.Fatalf("expected context line ' only': %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_NoTrailingNewline(t *testing.T) {
	req := &DiffRequest{
		Old: "line1\nline2",
		New: "line1\nchanged",
	}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	if !strings.Contains(result.Diff, "-line2") {
		t.Fatalf("expected -line2: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+changed") {
		t.Fatalf("expected +changed: %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_LargeInsert(t *testing.T) {
	old := "first\nlast\n"
	new_ := "first\na\nb\nc\nd\ne\nlast\n"
	req := &DiffRequest{Old: old, New: new_}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	for _, expected := range []string{"+a", "+b", "+c", "+d", "+e"} {
		if !strings.Contains(result.Diff, expected) {
			t.Fatalf("expected %s in diff: %s", expected, result.Diff)
		}
	}
	if !strings.Contains(result.Diff, " first") {
		t.Fatalf("expected context ' first': %s", result.Diff)
	}
	if !strings.Contains(result.Diff, " last") {
		t.Fatalf("expected context ' last': %s", result.Diff)
	}
}

func TestGenerateUnifiedDiff_LargeDelete(t *testing.T) {
	old := "first\na\nb\nc\nd\ne\nlast\n"
	new_ := "first\nlast\n"
	req := &DiffRequest{Old: old, New: new_}
	result := GenerateUnifiedDiff(req)

	if result.Equal {
		t.Fatal("expected equal=false")
	}
	for _, expected := range []string{"-a", "-b", "-c", "-d", "-e"} {
		if !strings.Contains(result.Diff, expected) {
			t.Fatalf("expected %s in diff: %s", expected, result.Diff)
		}
	}
}

func TestGenerateUnifiedDiff_HunkCounts(t *testing.T) {
	req := &DiffRequest{
		Old: "a\nb\nc\n",
		New: "a\nx\nc\ny\n",
	}
	result := GenerateUnifiedDiff(req)

	if !strings.Contains(result.Diff, "@@ -1,3 +1,4 @@") {
		t.Fatalf("expected hunk header @@ -1,3 +1,4 @@, got: %s", result.Diff)
	}
}

func TestLCS_SingleMatch(t *testing.T) {
	a := []string{"x", "a", "y"}
	b := []string{"m", "a", "n"}
	ops := lcs(a, b)

	eqCount := 0
	for _, op := range ops {
		if op == opEqual {
			eqCount++
		}
	}
	if eqCount != 1 {
		t.Fatalf("expected 1 equal op for 'a', got %d", eqCount)
	}
}

func TestLCS_OneElement(t *testing.T) {
	ops := lcs([]string{"a"}, []string{"a"})
	if len(ops) != 1 || ops[0] != opEqual {
		t.Fatalf("expected single equal op, got %v", ops)
	}

	ops2 := lcs([]string{"a"}, []string{"b"})
	hasEq := false
	for _, op := range ops2 {
		if op == opEqual {
			hasEq = true
		}
	}
	if hasEq {
		t.Fatal("expected no equal ops for different single elements")
	}
}

func TestBuildIdenticalDiff_MultiLine(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	diff := buildIdenticalDiff("a.txt", "b.txt", lines)

	if !strings.Contains(diff, "@@ -1,3 +1,3 @@") {
		t.Fatalf("bad hunk header for 3 lines: %s", diff)
	}
	for _, line := range lines {
		if !strings.Contains(diff, " "+line) {
			t.Fatalf("missing context line %q: %s", line, diff)
		}
	}
}

func TestFormatUnified_MixedOps(t *testing.T) {
	ops := []editOp{opEqual, opDelete, opInsert, opEqual}
	old := []string{"same1", "removed", "same2"}
	new_ := []string{"same1", "added", "same2"}
	result := formatUnified("a", "b", old, new_, ops)

	if !strings.Contains(result, " same1") {
		t.Fatalf("expected context ' same1': %s", result)
	}
	if !strings.Contains(result, "-removed") {
		t.Fatalf("expected -removed: %s", result)
	}
	if !strings.Contains(result, "+added") {
		t.Fatalf("expected +added: %s", result)
	}
	if !strings.Contains(result, " same2") {
		t.Fatalf("expected context ' same2': %s", result)
	}
}

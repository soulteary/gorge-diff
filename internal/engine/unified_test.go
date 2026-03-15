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

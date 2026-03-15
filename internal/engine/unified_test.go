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

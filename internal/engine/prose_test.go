package engine

import (
	"testing"
)

func TestGenerateProseDiff_Identical(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{Old: "hello world", New: "hello world"})

	if len(result.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d: %+v", len(result.Parts), result.Parts)
	}
	if result.Parts[0].Type != "=" {
		t.Fatalf("expected =, got %s", result.Parts[0].Type)
	}
}

func TestGenerateProseDiff_SimpleChange(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{Old: "hello", New: "world"})

	hasDel := false
	hasIns := false
	for _, p := range result.Parts {
		if p.Type == "-" {
			hasDel = true
		}
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasDel {
		t.Fatal("expected a deletion part")
	}
	if !hasIns {
		t.Fatal("expected an insertion part")
	}
}

func TestGenerateProseDiff_Addition(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "first paragraph",
		New: "first paragraph\n\nsecond paragraph",
	})

	hasIns := false
	for _, p := range result.Parts {
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasIns {
		t.Fatal("expected an insertion part for added paragraph")
	}
}

func TestGenerateProseDiff_Empty(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{Old: "", New: ""})
	if len(result.Parts) != 0 {
		t.Fatalf("expected 0 parts for empty inputs, got %d", len(result.Parts))
	}
}

func TestGenerateProseDiff_OldEmpty(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{Old: "", New: "hello"})

	if len(result.Parts) == 0 {
		t.Fatal("expected parts for non-empty new")
	}
	if result.Parts[0].Type != "+" {
		t.Fatalf("expected + part, got %s", result.Parts[0].Type)
	}
}

func TestGenerateProseDiff_NewEmpty(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{Old: "hello", New: ""})

	if len(result.Parts) == 0 {
		t.Fatal("expected parts for non-empty old")
	}
	if result.Parts[0].Type != "-" {
		t.Fatalf("expected - part, got %s", result.Parts[0].Type)
	}
}

func TestReorderParts(t *testing.T) {
	parts := []ProsePart{
		{Type: "+", Text: "new"},
		{Type: "-", Text: "old"},
		{Type: "=", Text: "same"},
	}
	result := reorderParts(parts)

	if len(result) < 2 {
		t.Fatalf("expected >= 2 parts, got %d", len(result))
	}

	// Deletions should come before insertions
	firstDel := -1
	firstIns := -1
	for i, p := range result {
		if p.Type == "-" && firstDel < 0 {
			firstDel = i
		}
		if p.Type == "+" && firstIns < 0 {
			firstIns = i
		}
	}
	if firstDel >= 0 && firstIns >= 0 && firstDel > firstIns {
		t.Fatal("expected deletions before insertions")
	}
}

func TestTrimApart(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"  hello  ", 3},
		{"  ", 1},
		{"hello  ", 2},
		{"  hello", 2},
	}
	for _, tt := range tests {
		got := trimApart(tt.input)
		if len(got) != tt.expected {
			t.Errorf("trimApart(%q) = %d parts, want %d; parts=%v", tt.input, len(got), tt.expected, got)
		}
	}
}

func TestSplitCorpus_Paragraphs(t *testing.T) {
	parts := splitCorpus("para1\n\npara2", 0)
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts for paragraph split")
	}
}

func TestSplitCorpus_Characters(t *testing.T) {
	parts := splitCorpus("abc", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 chars, got %d", len(parts))
	}
	if parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
		t.Fatalf("unexpected chars: %v", parts)
	}
}

func TestEditDistanceDiff_TooLarge(t *testing.T) {
	u := make([]string, 200)
	v := make([]string, 200)
	for i := range u {
		u[i] = "a"
		v[i] = "b"
	}
	_, tooLarge := editDistanceDiff(u, v, 2)
	if !tooLarge {
		t.Fatal("expected tooLarge=true for inputs exceeding maxEditDistance")
	}
}

func TestSmooth(t *testing.T) {
	// An isolated 's' surrounded by non-s should become 'x'
	ops := []byte{'d', 's', 'i'}
	result := smooth(ops)
	if result[1] != 'x' {
		t.Fatalf("expected smoothed 'x', got %c", result[1])
	}

	// Consecutive 's' should not be smoothed
	ops2 := []byte{'d', 's', 's', 'i'}
	result2 := smooth(ops2)
	if result2[1] != 's' || result2[2] != 's' {
		t.Fatalf("expected 's' preserved, got %c %c", result2[1], result2[2])
	}
}

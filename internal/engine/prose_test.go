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

func TestSmooth_AllSame(t *testing.T) {
	ops := []byte{'s', 's', 's'}
	result := smooth(ops)
	for i, c := range result {
		if c != 's' {
			t.Fatalf("ops[%d]: expected 's', got %c", i, c)
		}
	}
}

func TestSmooth_EdgePositions(t *testing.T) {
	ops := []byte{'s', 'i'}
	result := smooth(ops)
	if result[0] != 's' {
		t.Fatalf("leading 's' should not be smoothed, got %c", result[0])
	}

	ops2 := []byte{'d', 's'}
	result2 := smooth(ops2)
	if result2[1] != 's' {
		t.Fatalf("trailing 's' should not be smoothed, got %c", result2[1])
	}
}

func TestSplitCorpus_Sentences(t *testing.T) {
	parts := splitCorpus("Hello world. Goodbye world!", 1)
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts for sentence split")
	}
	joined := ""
	for _, p := range parts {
		joined += p
	}
	if joined != "Hello world. Goodbye world!" {
		t.Fatalf("joined parts should reconstruct input, got %q", joined)
	}
}

func TestSplitCorpus_Words(t *testing.T) {
	parts := splitCorpus("hello world foo", 2)
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts for word split")
	}
	joined := ""
	for _, p := range parts {
		joined += p
	}
	if joined != "hello world foo" {
		t.Fatalf("joined parts should reconstruct input, got %q", joined)
	}
}

func TestSplitCorpus_DefaultLevel(t *testing.T) {
	parts := splitCorpus("anything", 99)
	if len(parts) != 1 || parts[0] != "anything" {
		t.Fatalf("unexpected result for unknown level: %v", parts)
	}
}

func TestSplitCorpus_Empty(t *testing.T) {
	parts := splitCorpus("", 0)
	if len(parts) != 0 {
		t.Fatalf("expected empty parts for empty input, got %v", parts)
	}
}

func TestSplitChars_Unicode(t *testing.T) {
	parts := splitChars("你好")
	if len(parts) != 2 {
		t.Fatalf("expected 2 chars, got %d", len(parts))
	}
	if parts[0] != "你" || parts[1] != "好" {
		t.Fatalf("unexpected chars: %v", parts)
	}
}

func TestSplitChars_Empty(t *testing.T) {
	parts := splitChars("")
	if len(parts) != 0 {
		t.Fatalf("expected 0 chars for empty input, got %d", len(parts))
	}
}

func TestEditDistanceDiff_Normal(t *testing.T) {
	u := []string{"a", "b", "c"}
	v := []string{"a", "x", "c"}
	parts, tooLarge := editDistanceDiff(u, v, 2)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}

	hasEq, hasDel, hasIns := false, false, false
	for _, p := range parts {
		switch p.Type {
		case "=":
			hasEq = true
		case "-":
			hasDel = true
		case "+":
			hasIns = true
		}
	}
	if !hasEq || !hasDel || !hasIns {
		t.Fatalf("expected mix of =/-/+ parts, got %+v", parts)
	}
}

func TestEditDistanceDiff_Identical(t *testing.T) {
	u := []string{"a", "b", "c"}
	parts, tooLarge := editDistanceDiff(u, u, 2)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}
	for _, p := range parts {
		if p.Type != "=" {
			t.Fatalf("expected all = parts for identical input, got %+v", parts)
		}
	}
}

func TestEditDistanceDiff_InsertOnly(t *testing.T) {
	u := []string{}
	v := []string{"a", "b"}
	parts, tooLarge := editDistanceDiff(u, v, 1)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}
	for _, p := range parts {
		if p.Type != "+" {
			t.Fatalf("expected all + parts, got %+v", parts)
		}
	}
}

func TestEditDistanceDiff_DeleteOnly(t *testing.T) {
	u := []string{"a", "b"}
	v := []string{}
	parts, tooLarge := editDistanceDiff(u, v, 1)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}
	for _, p := range parts {
		if p.Type != "-" {
			t.Fatalf("expected all - parts, got %+v", parts)
		}
	}
}

func TestEditDistanceDiff_WithSmoothing(t *testing.T) {
	u := []string{"a", "b", "c"}
	v := []string{"x", "b", "y"}
	parts, tooLarge := editDistanceDiff(u, v, 2)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
}

func TestEditDistanceDiff_WithoutSmoothing(t *testing.T) {
	u := []string{"a", "b", "c"}
	v := []string{"x", "b", "y"}
	parts, tooLarge := editDistanceDiff(u, v, 1)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}

	hasEq := false
	for _, p := range parts {
		if p.Type == "=" && p.Text == "b" {
			hasEq = true
		}
	}
	if !hasEq {
		t.Fatalf("expected = part for 'b' at level 1, got %+v", parts)
	}
}

func TestIsVMatched(t *testing.T) {
	matches := []int{-1, 2, -1, 5}
	if !isVMatched(2, matches) {
		t.Fatal("expected vi=2 to be matched")
	}
	if !isVMatched(5, matches) {
		t.Fatal("expected vi=5 to be matched")
	}
	if isVMatched(0, matches) {
		t.Fatal("expected vi=0 to not be matched")
	}
	if isVMatched(3, matches) {
		t.Fatal("expected vi=3 to not be matched")
	}
}

func TestHashDiff_Reorder(t *testing.T) {
	u := []string{"aaa", "bbb", "ccc"}
	v := []string{"ccc", "bbb", "aaa"}
	parts := hashDiff(u, v)
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
}

func TestHashDiff_WithUnmatchedV(t *testing.T) {
	u := []string{"a", "b"}
	v := []string{"x", "a", "y", "b", "z"}
	parts := hashDiff(u, v)

	insCount := 0
	for _, p := range parts {
		if p.Type == "+" {
			insCount++
		}
	}
	if insCount != 3 {
		t.Fatalf("expected 3 insertions (x, y, z), got %d: %+v", insCount, parts)
	}
}

func TestHashDiff_AllNew(t *testing.T) {
	u := []string{"a"}
	v := []string{"b", "c"}
	parts := hashDiff(u, v)

	hasDel, hasIns := false, false
	for _, p := range parts {
		if p.Type == "-" {
			hasDel = true
		}
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasDel || !hasIns {
		t.Fatalf("expected both - and + parts, got %+v", parts)
	}
}

func TestCombineRuns_WithPrefix(t *testing.T) {
	oRun := []ProsePart{{Type: "-", Text: " hello"}}
	nRun := []ProsePart{{Type: "+", Text: " world"}}
	result := combineRuns(oRun, nRun)

	if len(result) < 2 {
		t.Fatalf("expected >= 2 parts, got %d: %+v", len(result), result)
	}
	if result[0].Type != "=" || result[0].Text != " " {
		t.Fatalf("expected prefix '=' part with space, got %+v", result[0])
	}
}

func TestCombineRuns_WithSuffix(t *testing.T) {
	oRun := []ProsePart{{Type: "-", Text: "hello."}}
	nRun := []ProsePart{{Type: "+", Text: "world."}}
	result := combineRuns(oRun, nRun)

	last := result[len(result)-1]
	if last.Type != "=" || last.Text != "." {
		t.Fatalf("expected suffix '=' part with '.', got %+v", last)
	}
}

func TestCombineRuns_NoCommonLayoutChars(t *testing.T) {
	oRun := []ProsePart{{Type: "-", Text: "abc"}}
	nRun := []ProsePart{{Type: "+", Text: "xyz"}}
	result := combineRuns(oRun, nRun)

	if len(result) != 2 {
		t.Fatalf("expected 2 parts (- and +), got %d: %+v", len(result), result)
	}
}

func TestCombineRuns_OnlyDelete(t *testing.T) {
	oRun := []ProsePart{{Type: "-", Text: "removed"}}
	result := combineRuns(oRun, nil)

	if len(result) != 1 || result[0].Type != "-" {
		t.Fatalf("expected single - part, got %+v", result)
	}
}

func TestCombineRuns_OnlyInsert(t *testing.T) {
	result := combineRuns(nil, []ProsePart{{Type: "+", Text: "added"}})

	if len(result) != 1 || result[0].Type != "+" {
		t.Fatalf("expected single + part, got %+v", result)
	}
}

func TestMergeRunText(t *testing.T) {
	parts := []ProsePart{
		{Text: "hello"},
		{Text: " "},
		{Text: "world"},
	}
	got := mergeRunText(parts)
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestMergeRunText_Empty(t *testing.T) {
	got := mergeRunText(nil)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestMin3(t *testing.T) {
	tests := []struct {
		a, b, c  int
		expected int
	}{
		{1, 2, 3, 1},
		{3, 1, 2, 1},
		{3, 2, 1, 1},
		{1, 1, 1, 1},
		{1, 1, 2, 1},
		{2, 1, 1, 1},
	}
	for _, tt := range tests {
		got := min3(tt.a, tt.b, tt.c)
		if got != tt.expected {
			t.Errorf("min3(%d,%d,%d) = %d, want %d", tt.a, tt.b, tt.c, got, tt.expected)
		}
	}
}

func TestReorderParts_OnlyEquals(t *testing.T) {
	parts := []ProsePart{
		{Type: "=", Text: "hello"},
		{Type: "=", Text: " world"},
	}
	result := reorderParts(parts)
	if len(result) != 1 {
		t.Fatalf("expected merged into 1 part, got %d: %+v", len(result), result)
	}
	if result[0].Text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result[0].Text)
	}
}

func TestReorderParts_Empty(t *testing.T) {
	result := reorderParts(nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 parts for nil input, got %d", len(result))
	}
}

func TestReorderParts_MixedRun(t *testing.T) {
	parts := []ProsePart{
		{Type: "+", Text: "new1"},
		{Type: "-", Text: "old1"},
		{Type: "+", Text: "new2"},
		{Type: "-", Text: "old2"},
	}
	result := reorderParts(parts)

	firstDel, firstIns := -1, -1
	for i, p := range result {
		if p.Type == "-" && firstDel < 0 {
			firstDel = i
		}
		if p.Type == "+" && firstIns < 0 {
			firstIns = i
		}
	}
	if firstDel >= 0 && firstIns >= 0 && firstDel > firstIns {
		t.Fatal("expected all deletions before insertions")
	}
}

func TestGenerateProseDiff_MultipleParagraphs(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "para1\n\npara2\n\npara3",
		New: "para1\n\nchanged\n\npara3",
	})

	hasEq, hasDel, hasIns := false, false, false
	for _, p := range result.Parts {
		switch p.Type {
		case "=":
			hasEq = true
		case "-":
			hasDel = true
		case "+":
			hasIns = true
		}
	}
	if !hasEq {
		t.Fatal("expected = parts for unchanged paragraphs")
	}
	if !hasDel || !hasIns {
		t.Fatal("expected -/+ parts for changed paragraph")
	}
}

func TestGenerateProseDiff_WordLevelChange(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "the quick brown fox",
		New: "the slow brown fox",
	})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	hasChange := false
	for _, p := range result.Parts {
		if p.Type == "-" || p.Type == "+" {
			hasChange = true
		}
	}
	if !hasChange {
		t.Fatal("expected change parts for word-level diff")
	}
}

func TestGenerateProseDiff_CharLevelChange(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "abc",
		New: "axc",
	})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
}

func TestStitchPieces_LevelTwo(t *testing.T) {
	parts := []string{"hello", "world"}
	delims := []string{" "}
	result := stitchPieces(parts, delims, 2)
	if len(result) == 0 {
		t.Fatal("expected non-empty results")
	}
}

func TestStitchPieces_TrailingEmpty(t *testing.T) {
	parts := []string{"hello", ""}
	delims := []string{"\n"}
	result := stitchPieces(parts, delims, 0)
	for _, r := range result {
		if r == "" {
			t.Fatal("trailing empty string should have been trimmed")
		}
	}
}

func TestBuildProseDiff_BothBlocksEmpty(t *testing.T) {
	result := buildProseDiff("", "", 0)
	if len(result) != 0 {
		t.Fatalf("expected 0 parts for empty inputs, got %d", len(result))
	}
}

func TestBuildProseDiff_Level3(t *testing.T) {
	result := buildProseDiff("abc", "axc", 3)
	if len(result) == 0 {
		t.Fatal("expected non-empty result at character level")
	}
}

func TestBuildProseDiff_TooLargeSkipsRecursion(t *testing.T) {
	var oldParts, newParts []string
	for i := 0; i < 200; i++ {
		oldParts = append(oldParts, "old")
		newParts = append(newParts, "new")
	}
	old := ""
	for i, p := range oldParts {
		if i > 0 {
			old += ". "
		}
		old += p
	}
	new_ := ""
	for i, p := range newParts {
		if i > 0 {
			new_ += ". "
		}
		new_ += p
	}

	result := buildProseDiff(old, new_, 1)
	if len(result) == 0 {
		t.Fatal("expected non-empty result for tooLarge input")
	}
	hasDel, hasIns := false, false
	for _, p := range result {
		if p.Type == "-" {
			hasDel = true
		}
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasDel || !hasIns {
		t.Fatalf("expected -/+ parts for tooLarge fallback, got %+v", result)
	}
}

func TestStitchPieces_TrailingEmptyAtLevelTwo(t *testing.T) {
	parts := []string{"hello", "world", ""}
	delims := []string{" ", " "}
	result := stitchPieces(parts, delims, 2)
	for _, r := range result {
		if r == "" {
			t.Fatal("trailing empty string should have been trimmed at level 2")
		}
	}
}

func TestBuildProseDiff_EmptyChangeBlock(t *testing.T) {
	result := buildProseDiff("\n\n", "\n\n", 0)
	for _, p := range result {
		if (p.Type == "-" || p.Type == "+") && p.Text == "" {
			t.Fatal("should not produce empty change parts")
		}
	}
}

func TestGenerateProseDiff_ParagraphDeletion(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "para1\n\npara2\n\npara3",
		New: "para1\n\npara3",
	})

	hasDel := false
	for _, p := range result.Parts {
		if p.Type == "-" {
			hasDel = true
		}
	}
	if !hasDel {
		t.Fatal("expected deletion for removed paragraph")
	}
}

func TestGenerateProseDiff_SentenceLevelChange(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "First sentence. Second sentence. Third sentence.",
		New: "First sentence. Changed sentence. Third sentence.",
	})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	hasEq, hasChange := false, false
	for _, p := range result.Parts {
		if p.Type == "=" {
			hasEq = true
		}
		if p.Type == "-" || p.Type == "+" {
			hasChange = true
		}
	}
	if !hasEq || !hasChange {
		t.Fatalf("expected = and -/+ parts for sentence-level change, got %+v", result.Parts)
	}
}

func TestGenerateProseDiff_LongTextWithSmallChange(t *testing.T) {
	old := "The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs."
	new_ := "The quick brown fox jumps over the lazy cat. " +
		"Pack my box with five dozen liquor jugs."
	result := GenerateProseDiff(&ProseRequest{Old: old, New: new_})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	totalText := ""
	for _, p := range result.Parts {
		if p.Type == "=" || p.Type == "+" {
			totalText += p.Text
		}
	}
	if totalText != new_ {
		t.Fatalf("reconstructed new text mismatch: got %q", totalText)
	}
}

func TestGenerateProseDiff_UnicodeContent(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "你好世界",
		New: "你好地球",
	})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts for unicode diff")
	}
	hasDel, hasIns := false, false
	for _, p := range result.Parts {
		if p.Type == "-" {
			hasDel = true
		}
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasDel || !hasIns {
		t.Fatalf("expected -/+ parts for unicode change, got %+v", result.Parts)
	}
}

func TestGenerateProseDiff_OnlyWhitespaceChange(t *testing.T) {
	result := GenerateProseDiff(&ProseRequest{
		Old: "hello world",
		New: "hello  world",
	})

	if len(result.Parts) == 0 {
		t.Fatal("expected non-empty parts for whitespace change")
	}
}

func TestGenerateProseDiff_TextReconstruction(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new_ string
	}{
		{"word swap", "the quick fox", "the slow fox"},
		{"paragraph add", "para1", "para1\n\npara2"},
		{"sentence change", "Hello world. Goodbye moon.", "Hello world. Goodbye sun."},
		{"complete replace", "alpha", "beta"},
		{"multi paragraph change", "A\n\nB\n\nC", "A\n\nX\n\nC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateProseDiff(&ProseRequest{Old: tt.old, New: tt.new_})
			var oldRecon, newRecon string
			for _, p := range result.Parts {
				switch p.Type {
				case "=":
					oldRecon += p.Text
					newRecon += p.Text
				case "-":
					oldRecon += p.Text
				case "+":
					newRecon += p.Text
				}
			}
			if oldRecon != tt.old {
				t.Errorf("old reconstruction mismatch: got %q, want %q", oldRecon, tt.old)
			}
			if newRecon != tt.new_ {
				t.Errorf("new reconstruction mismatch: got %q, want %q", newRecon, tt.new_)
			}
		})
	}
}

func TestHashDiff_IdenticalParts(t *testing.T) {
	u := []string{"a", "b", "c"}
	v := []string{"a", "b", "c"}
	parts := hashDiff(u, v)

	for _, p := range parts {
		if p.Type != "=" {
			t.Fatalf("expected all = parts for identical input, got %+v", parts)
		}
	}
}

func TestHashDiff_Empty(t *testing.T) {
	parts := hashDiff(nil, nil)
	if len(parts) != 0 {
		t.Fatalf("expected 0 parts for empty inputs, got %d", len(parts))
	}
}

func TestHashDiff_DuplicateContent(t *testing.T) {
	u := []string{"x", "x", "x"}
	v := []string{"x", "y", "x"}
	parts := hashDiff(u, v)

	hasIns := false
	for _, p := range parts {
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasIns {
		t.Fatalf("expected + part for inserted 'y', got %+v", parts)
	}
}

func TestCombineRuns_PrefixAndSuffix(t *testing.T) {
	oRun := []ProsePart{{Type: "-", Text: " hello."}}
	nRun := []ProsePart{{Type: "+", Text: " world."}}
	result := combineRuns(oRun, nRun)

	if len(result) < 3 {
		t.Fatalf("expected >= 3 parts (prefix + changes + suffix), got %d: %+v", len(result), result)
	}
	if result[0].Type != "=" || result[0].Text != " " {
		t.Fatalf("expected prefix = ' ', got %+v", result[0])
	}
	last := result[len(result)-1]
	if last.Type != "=" || last.Text != "." {
		t.Fatalf("expected suffix = '.', got %+v", last)
	}
}

func TestEditDistanceDiff_SingleElement(t *testing.T) {
	u := []string{"a"}
	v := []string{"b"}
	parts, tooLarge := editDistanceDiff(u, v, 1)
	if tooLarge {
		t.Fatal("expected tooLarge=false")
	}
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts")
	}
	hasDel, hasIns := false, false
	for _, p := range parts {
		if p.Type == "-" {
			hasDel = true
		}
		if p.Type == "+" {
			hasIns = true
		}
	}
	if !hasDel || !hasIns {
		t.Fatalf("expected -/+ for single element replacement, got %+v", parts)
	}
}

func TestEditDistanceDiff_LargeButUnderThreshold(t *testing.T) {
	u := make([]string, 128)
	v := make([]string, 128)
	for i := range u {
		u[i] = "same"
		v[i] = "same"
	}
	v[64] = "changed"
	parts, tooLarge := editDistanceDiff(u, v, 1)
	if tooLarge {
		t.Fatal("expected tooLarge=false for exactly 128 elements")
	}
	hasChange := false
	for _, p := range parts {
		if p.Type == "-" || p.Type == "+" {
			hasChange = true
		}
	}
	if !hasChange {
		t.Fatal("expected change parts for modified element")
	}
}

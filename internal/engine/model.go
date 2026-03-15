package engine

// DiffRequest is the input for unified diff generation.
type DiffRequest struct {
	Old       string `json:"old"`
	New       string `json:"new"`
	OldName   string `json:"oldName,omitempty"`
	NewName   string `json:"newName,omitempty"`
	Normalize bool   `json:"normalize,omitempty"`
}

// DiffResult is the output of unified diff generation.
type DiffResult struct {
	Diff  string `json:"diff"`
	Equal bool   `json:"equal"`
}

// ProseRequest is the input for prose diff generation.
type ProseRequest struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// ProsePart represents one segment in a prose diff result.
type ProsePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ProseResult is the output of prose diff generation.
type ProseResult struct {
	Parts []ProsePart `json:"parts"`
}

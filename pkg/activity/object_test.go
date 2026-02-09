package activity

import "testing"

func TestParseCompositeObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantID   string
		wantOK   bool
	}{
		{
			name:     "valid type id",
			input:    "page:123",
			wantType: "page",
			wantID:   "123",
			wantOK:   true,
		},
		{
			name:     "no colon",
			input:    "page",
			wantType: "page",
			wantID:   "",
			wantOK:   false,
		},
		{
			name:     "multiple colons splits once",
			input:    "user:abc:def",
			wantType: "user",
			wantID:   "abc:def",
			wantOK:   true,
		},
		{
			name:     "empty type",
			input:    ":123",
			wantType: "",
			wantID:   "123",
			wantOK:   false,
		},
		{
			name:     "empty id",
			input:    "page:",
			wantType: "page",
			wantID:   "",
			wantOK:   false,
		},
		{
			name:     "whitespace trimmed",
			input:    "  page  :  123  ",
			wantType: "page",
			wantID:   "123",
			wantOK:   true,
		},
		{
			name:     "empty input",
			input:    "  ",
			wantType: "",
			wantID:   "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotID, gotOK := ParseCompositeObject(tt.input)
			if gotType != tt.wantType || gotID != tt.wantID || gotOK != tt.wantOK {
				t.Fatalf(
					"ParseCompositeObject(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input,
					gotType,
					gotID,
					gotOK,
					tt.wantType,
					tt.wantID,
					tt.wantOK,
				)
			}
		})
	}
}

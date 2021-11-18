package change

import "testing"

func TestHasChangeID(t *testing.T) {
	tests := []struct {
		expected bool
		message  string
	}{
		{false, ``},
		{false, "hello"},
		{true, "Change-ID: hello"},
		{true, "Change-ID : hello"},
		{true, "Change-ID :hello"},
		{false, " Change-ID :hello"},
		{true, "pants\nChange-ID: hello\npants"},
	}

	for _, tc := range tests {
		result := hasChangeID.MatchString(tc.message)
		if tc.expected != result {
			t.Errorf("Match(%q) = %v; want %v", tc.message, result, tc.expected)
		}
	}
}

func TestHasTrailers(t *testing.T) {
	tests := []struct {
		expected bool
		message  string
	}{
		{false, ``},
		{false, "hello"},
		{false, "Change-ID: hello"},
		{true, "\nChange-ID: hello"},
		{true, "\nChange-ID : hello"},
		{true, "\nChange-ID :hello"},
		{false, "pants\nChange-ID: hello\npants"},
		{false, "pants\nChange-ID: hello"},
		{true, "pants\n\nChange-ID: hello"},
		{true, "pants\n\nChange-ID: hello\n"},
		{true, "pants\n\nTrailer1: v1\nTrailer2: v2\n"},
		{true, "pants\n\nTrailer1: v1\nTrailer2: v2"},
		{true, "pants\n\nTrailer1: v1\nTrailer2: v2\n v2more"},
		{true, "pants\n\nTrailer1: v1\nTrailer2: v2\n v2more\n"},
		{true, "\n\nTrailer1: v1\nTrailer2: v2\n v2more\n"},
		{true, "\nTrailer1: v1\nTrailer2: v2\n v2more\n"},
		{false, "Trailer1: v1\nTrailer2: v2\n v2more\npants\n"},
		{false, ""},
		{true, "hello\nworld\n\nTrailer 1: value1"},
		{false, "Trailer 1: value1"},
	}

	for _, tc := range tests {
		result := hasTrailers.MatchString(tc.message)
		if tc.expected != result {
			t.Errorf("Match(%q) = %v; want %v", tc.message, result, tc.expected)
		}
	}
}

func TestAppendChangeID(t *testing.T) {
	changeID := "12345"

	tests := []struct {
		message  string
		expected string
	}{
		{"", "Change-ID: 12345\n"},
		{"hello", "hello\n\nChange-ID: 12345\n"},
		{"hello\n", "hello\n\nChange-ID: 12345\n"},
		{"hello\nworld\n", "hello\nworld\n\nChange-ID: 12345\n"},
		{"hello\nworld\n\nTrailer 1: value1", "hello\nworld\n\nTrailer 1: value1\nChange-ID: 12345\n"},
		{"hello\nworld\n\nTrailer 1: value1\n", "hello\nworld\n\nTrailer 1: value1\nChange-ID: 12345\n"},
	}

	for _, tc := range tests {
		result := appendChangeID(tc.message, changeID)
		if tc.expected != result {
			t.Errorf("appendChangeID(%q) = %q; want %q", tc.message, result, tc.expected)
		}
	}
}

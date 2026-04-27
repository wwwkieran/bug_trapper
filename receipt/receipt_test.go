package receipt

import (
	"strings"
	"testing"
	"time"
)

func TestAllLinesMaxWidth(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Monarch Butterfly",
		Description:   "A beautiful orange and black butterfly that travels thousands of miles every year!",
		ASCIIArt:      strings.Repeat("X", ReceiptWidth) + "\n" + strings.Repeat("X", ReceiptWidth),
		SightingCount: 1,
		Timestamp:     time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
	}
	result := Format(data)
	for i, line := range strings.Split(result, "\n") {
		if len(line) > ReceiptWidth {
			t.Errorf("line %d exceeds %d chars (%d): %q", i+1, ReceiptWidth, len(line), line)
		}
	}
}

func TestHeaderPresent(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Ant",
		Description:   "Tiny but strong!",
		ASCIIArt:      "ooo",
		SightingCount: 1,
		Timestamp:     time.Now(),
	}
	result := Format(data)
	if !strings.Contains(result, "BUG TRAPPER") {
		t.Error("receipt should contain BUG TRAPPER header")
	}
}

func TestOrganismNamePresent(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Ladybug",
		Description:   "A small red bug with spots.",
		ASCIIArt:      "o",
		SightingCount: 1,
		Timestamp:     time.Now(),
	}
	result := Format(data)
	if !strings.Contains(result, "Ladybug") {
		t.Error("receipt should contain organism name")
	}
}

func TestDescriptionWordWrap(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Bee",
		Description:   "Bees are amazing insects that make honey and help flowers grow by spreading pollen from plant to plant.",
		ASCIIArt:      "o",
		SightingCount: 1,
		Timestamp:     time.Now(),
	}
	result := Format(data)
	for i, line := range strings.Split(result, "\n") {
		if len(line) > ReceiptWidth {
			t.Errorf("line %d exceeds width: %q", i+1, line)
		}
	}
	if !strings.Contains(result, "pollen") {
		t.Error("receipt should contain full description")
	}
}

func TestCountMessageFirstSighting(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Ant",
		Description:   "Tiny.",
		ASCIIArt:      "o",
		SightingCount: 1,
		Timestamp:     time.Now(),
	}
	result := Format(data)
	if strings.Contains(result, "time you've found") {
		t.Error("should NOT show count message for first sighting")
	}
}

func TestCountMessageRepeatSighting(t *testing.T) {
	data := ReceiptData{
		OrganismName:  "Ant",
		Description:   "Tiny.",
		ASCIIArt:      "o",
		SightingCount: 3,
		Timestamp:     time.Now(),
	}
	result := Format(data)
	if !strings.Contains(result, "3rd") {
		t.Errorf("should show '3rd' in count message, got:\n%s", result)
	}
	if !strings.Contains(result, "found this organism") {
		t.Error("should show count message for repeat sighting")
	}
}

func TestOrdinals(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "1st"}, {2, "2nd"}, {3, "3rd"}, {4, "4th"},
		{11, "11th"}, {12, "12th"}, {13, "13th"},
		{21, "21st"}, {22, "22nd"}, {23, "23rd"},
		{101, "101st"}, {111, "111th"}, {112, "112th"},
	}
	for _, tc := range cases {
		got := Ordinal(tc.n)
		if got != tc.want {
			t.Errorf("Ordinal(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestWordWrap(t *testing.T) {
	input := "The quick brown fox jumps over the lazy dog near the river bank"
	wrapped := WordWrap(input, 20)
	for i, line := range strings.Split(wrapped, "\n") {
		if len(line) > 20 {
			t.Errorf("line %d exceeds 20 chars: %q", i+1, line)
		}
	}
}

func TestCenterText(t *testing.T) {
	result := CenterText("HI", 10)
	if len(result) > 10 {
		t.Errorf("centered text exceeds width: %q", result)
	}
	if !strings.Contains(result, "HI") {
		t.Error("centered text should contain original text")
	}
	// Should have leading spaces
	if result[0] != ' ' {
		t.Error("centered text should have leading spaces")
	}
}

func TestTimestampPresent(t *testing.T) {
	ts := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	data := ReceiptData{
		OrganismName:  "Ant",
		Description:   "Tiny.",
		ASCIIArt:      "o",
		SightingCount: 1,
		Timestamp:     ts,
	}
	result := Format(data)
	if !strings.Contains(result, "2025") {
		t.Error("receipt should contain timestamp year")
	}
}

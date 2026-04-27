package receipt

import (
	"fmt"
	"strings"
	"time"
)

const ReceiptWidth = 32

type ReceiptData struct {
	OrganismName  string
	Description   string
	ASCIIArt      string
	SightingCount int
	Timestamp     time.Time
}

func Format(data ReceiptData) string {
	return FormatHeader() + formatName(data.OrganismName) + FormatDescription(data.Description) + formatArt(data.ASCIIArt) + FormatBottom(data)
}

// FormatHeader returns the "BUG TRAPPER" banner and trailing blank line.
func FormatHeader() string {
	var b strings.Builder
	divider := strings.Repeat("=", ReceiptWidth)
	b.WriteString(divider + "\n")
	b.WriteString(CenterText("BUG TRAPPER", ReceiptWidth) + "\n")
	b.WriteString(divider + "\n")
	b.WriteString("\n")
	return b.String()
}

// FormatDescription returns the description block with a leading and trailing
// thin divider.
func FormatDescription(description string) string {
	var b strings.Builder
	thinDiv := strings.Repeat("-", ReceiptWidth)
	b.WriteString(thinDiv + "\n")
	b.WriteString(WordWrap(description, ReceiptWidth) + "\n")
	b.WriteString(thinDiv + "\n")
	return b.String()
}

func formatName(name string) string {
	return WordWrap("Name: "+name, ReceiptWidth) + "\n"
}

// FormatBottom returns the count message + timestamp + trailing divider.
func FormatBottom(data ReceiptData) string {
	var b strings.Builder
	divider := strings.Repeat("=", ReceiptWidth)

	b.WriteString(strings.Repeat("-", ReceiptWidth) + "\n")

	if data.SightingCount > 1 {
		msg := fmt.Sprintf("This is the %s time you've found this organism!", Ordinal(data.SightingCount))
		b.WriteString(WordWrap(msg, ReceiptWidth) + "\n")
		b.WriteString("\n")
	}

	ts := data.Timestamp.Format("2006-01-02 15:04:05")
	b.WriteString(CenterText(ts, ReceiptWidth) + "\n")
	b.WriteString(divider + "\n")

	return b.String()
}

func formatArt(art string) string {
	var b strings.Builder
	for _, line := range strings.Split(art, "\n") {
		if len(line) > ReceiptWidth {
			line = line[:ReceiptWidth]
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func WordWrap(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			// If a single word exceeds width, break it
			for len(word) > width {
				lines = append(lines, word[:width])
				word = word[width:]
			}
			currentLine = word
		}
	}
	lines = append(lines, currentLine)
	return strings.Join(lines, "\n")
}

func CenterText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	pad := (width - len(text)) / 2
	return strings.Repeat(" ", pad) + text
}

func Ordinal(n int) string {
	suffix := "th"
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 13 {
		// special case: 11th, 12th, 13th
	} else {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

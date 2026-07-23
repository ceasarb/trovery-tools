package console

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Header prints bold cyan text.
func Header(text string) {
	fmt.Fprintln(os.Stderr, headerStyle.Render(text))
}

// Success prints green text with a checkmark.
func Success(text string) {
	fmt.Fprintln(os.Stderr, successStyle.Render("✓ "+text))
}

// Error prints red text with an X.
func Error(text string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("✗ "+text))
}

// Warning prints yellow text.
func Warning(text string) {
	fmt.Fprintln(os.Stderr, warningStyle.Render("! "+text))
}

// Dim prints gray secondary text.
func Dim(text string) {
	fmt.Fprintln(os.Stderr, dimStyle.Render(text))
}

// Table prints a simple aligned table to stderr.
func Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths.
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header.
	var hdr strings.Builder
	for i, h := range headers {
		if i > 0 {
			hdr.WriteString("  ")
		}
		hdr.WriteString(fmt.Sprintf("%-*s", widths[i], h))
	}
	fmt.Fprintln(os.Stderr, headerStyle.Render(hdr.String()))

	// Print separator.
	var sep strings.Builder
	for i, w := range widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(strings.Repeat("─", w))
	}
	fmt.Fprintln(os.Stderr, dimStyle.Render(sep.String()))

	// Print rows.
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(fmt.Sprintf("%-*s", widths[i], cell))
		}
		fmt.Fprintln(os.Stderr, line.String())
	}
}

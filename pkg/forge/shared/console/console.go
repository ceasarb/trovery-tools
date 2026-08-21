package console

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Brand colors
	Purple = lipgloss.Color("#7C3AED")
	Green  = lipgloss.Color("#10B981")
	Red    = lipgloss.Color("#EF4444")
	Yellow = lipgloss.Color("#F59E0B")
	Blue   = lipgloss.Color("#3B82F6")
	Gray   = lipgloss.Color("#6B7280")

	// Styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Green)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Red)

	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Yellow)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Blue)

	DimStyle = lipgloss.NewStyle().
			Foreground(Gray)
)

func Header(msg string) {
	fmt.Println(HeaderStyle.Render(msg))
}

func Success(msg string) {
	fmt.Println(SuccessStyle.Render("✓ " + msg))
}

func Error(msg string) {
	fmt.Fprintln(os.Stderr, ErrorStyle.Render("✗ "+msg))
}

func Warning(msg string) {
	fmt.Println(WarningStyle.Render("! " + msg))
}

func Info(msg string) {
	fmt.Println(InfoStyle.Render(msg))
}

func Dim(msg string) {
	fmt.Println(DimStyle.Render(msg))
}

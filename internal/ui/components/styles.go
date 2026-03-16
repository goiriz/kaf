package components

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	PrimaryColor   lipgloss.Color
	SecondaryColor lipgloss.Color
	DimColor       lipgloss.Color
	ErrorColor     lipgloss.Color
	WarningColor   lipgloss.Color
	HighlightColor lipgloss.Color
	HeaderBgColor  lipgloss.Color
	HeaderFgColor  lipgloss.Color

	TitleStyle         lipgloss.Style
	InactiveTitleStyle lipgloss.Style
	HelpStyle          lipgloss.Style
	SelectedStyle      lipgloss.Style
	DeletingStyle      lipgloss.Style
	InternalStyle      lipgloss.Style
	ErrorStyle         lipgloss.Style
	InfoStyle          lipgloss.Style
	ToastStyle         lipgloss.Style
	HighlightStyle     lipgloss.Style
	DimStyle           lipgloss.Style
	SecondaryStyle     lipgloss.Style
}

var DefaultTheme = Theme{
	PrimaryColor:   lipgloss.Color("170"),
	SecondaryColor: lipgloss.Color("62"),
	DimColor:       lipgloss.Color("241"),
	ErrorColor:     lipgloss.Color("196"),
	WarningColor:   lipgloss.Color("208"),
	HighlightColor: lipgloss.Color("226"),
	HeaderBgColor:  lipgloss.Color("62"),
	HeaderFgColor:  lipgloss.Color("230"),
}

func init() {
	DefaultTheme.TitleStyle = lipgloss.NewStyle().
		Background(DefaultTheme.HeaderBgColor).
		Foreground(DefaultTheme.HeaderFgColor).
		Padding(0, 1).
		Bold(true)

	DefaultTheme.InactiveTitleStyle = lipgloss.NewStyle().
		Background(DefaultTheme.DimColor).
		Foreground(lipgloss.Color("250")).
		Padding(0, 1)

	DefaultTheme.HelpStyle = lipgloss.NewStyle().Foreground(DefaultTheme.DimColor)
	DefaultTheme.SelectedStyle = lipgloss.NewStyle().Foreground(DefaultTheme.PrimaryColor).Bold(true)
	DefaultTheme.DeletingStyle = lipgloss.NewStyle().Foreground(DefaultTheme.DimColor).Strikethrough(true)
	DefaultTheme.InternalStyle = lipgloss.NewStyle().Foreground(DefaultTheme.WarningColor).Italic(true)
	DefaultTheme.ErrorStyle = lipgloss.NewStyle().Foreground(DefaultTheme.ErrorColor)
	DefaultTheme.InfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Background(lipgloss.Color("62")).Padding(0, 1).Bold(true)
	DefaultTheme.ToastStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Background(DefaultTheme.ErrorColor).Padding(0, 1).Bold(true)
	DefaultTheme.HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(DefaultTheme.HighlightColor)
	DefaultTheme.DimStyle = lipgloss.NewStyle().Foreground(DefaultTheme.DimColor)
	DefaultTheme.SecondaryStyle = lipgloss.NewStyle().Foreground(DefaultTheme.SecondaryColor).Bold(true)
}

// GlobalTheme is used throughout the app. Can be swapped for testing/config.
var GlobalTheme = DefaultTheme

// Backward compatibility aliases (to be removed later)
var (
	PrimaryColor   = GlobalTheme.PrimaryColor
	SecondaryColor = GlobalTheme.SecondaryColor
	DimColor       = GlobalTheme.DimColor
	ErrorColor     = GlobalTheme.ErrorColor
	TitleStyle     = GlobalTheme.TitleStyle
	InactiveTitleStyle = GlobalTheme.InactiveTitleStyle
	HelpStyle      = GlobalTheme.HelpStyle
	SelectedStyle  = GlobalTheme.SelectedStyle
	DeletingStyle  = GlobalTheme.DeletingStyle
	InternalStyle  = GlobalTheme.InternalStyle
)

func TableHeader(text string) string {
	return GlobalTheme.SecondaryStyle.Render(text)
}

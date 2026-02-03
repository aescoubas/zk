package main

import "github.com/charmbracelet/lipgloss"

// Gruvbox Colors
const (
    GruvboxBg       = "#282828"
    GruvboxFg       = "#ebdbb2"
    GruvboxRed      = "#cc241d"
    GruvboxGreen    = "#98971a"
    GruvboxYellow   = "#d79921"
    GruvboxBlue     = "#458588"
    GruvboxPurple   = "#b16286"
    GruvboxAqua     = "#689d6a"
    GruvboxGray     = "#a89984"
    GruvboxOrange   = "#d65d0e"
    
    // Bright
    GruvboxRedBright    = "#fb4934"
    GruvboxGreenBright  = "#b8bb26"
    GruvboxYellowBright = "#fabd2f"
    GruvboxBlueBright   = "#83a598"
    GruvboxPurpleBright = "#d3869b"
    GruvboxAquaBright   = "#8ec07c"
    GruvboxGrayBright   = "#928374"
    GruvboxOrangeBright = "#fe8019"
)

// Lipgloss Styles
var (
    subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
    highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: GruvboxOrangeBright}
    special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: GruvboxGreenBright}
)

// Glamour Style JSON
const gruvboxStyle = `
{
  "document": {
    "margin": 2
  },
  "block_quote": {
    "color": "#928374",
    "indent": 4,
    "italic": true
  },
  "code_block": {
    "color": "#ebdbb2",
    "margin": 2
  },
  "h1": {
    "color": "#fabd2f",
    "bold": true,
    "underline": true
  },
  "h2": {
    "color": "#b8bb26",
    "bold": true
  },
  "h3": {
    "color": "#83a598",
    "bold": true
  },
  "h4": {
    "color": "#fabd2f"
  },
  "h5": {
    "color": "#83a598"
  },
  "h6": {
    "color": "#d3869b"
  },
  "link": {
    "color": "#83a598",
    "underline": true
  },
  "link_text": {
    "color": "#83a598",
    "bold": true
  },
  "list": {
    "color": "#ebdbb2"
  },
  "strong": {
    "color": "#fe8019",
    "bold": true
  },
  "text": {
    "color": "#ebdbb2"
  }
}
`

package styles

import "github.com/charmbracelet/lipgloss"

// Styles exported so other UI packages can consume a common set of styles.
// These are stubs that mirror the names used in the existing single-file UI.
var (
	TitleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("63")).Bold(true)
	StatsStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Background(lipgloss.Color("235"))
	CatIOStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	CatFSStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("149"))
	CatNetStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	CatMemStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	CatProcStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	CatSigStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	CatOthStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	HeaderStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	ActiveSortStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("118")).Bold(true)
	RowStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ErrRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))            // row with >0 errors but error rate below the warning threshold
	HotRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // row with very high error rate (>= 50 %)
	SlowRowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("227")).Bold(true) // row whose avg latency exceeds the warning threshold
	BarFillStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	ErrNumStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SlowAvgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("227"))
	DivStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	FooterStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	FilterStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	AlertStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	SelectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255")).Bold(true)
	DetailTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("25")).Bold(true)
	DetailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	DetailValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	DetailDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	DetailCodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("149"))
)

// Package style adds ANSI colors and emphasis to dockshim's own messages.
package style

// Style adds ANSI SGR codes to text. The zero Style is disabled: it returns text unchanged.
type Style struct {
	Enabled bool
}

func (s Style) Error(t string) string   { return s.sgr("1;31", t) }
func (s Style) Warning(t string) string { return s.sgr("1;33", t) }
func (s Style) Success(t string) string { return s.sgr("32", t) }
func (s Style) Title(t string) string   { return s.sgr("1", t) }
func (s Style) Key(t string) string     { return s.sgr("36", t) }
func (s Style) Faint(t string) string   { return s.sgr("2", t) }

func (s Style) sgr(code, t string) string {
	if !s.Enabled || t == "" {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

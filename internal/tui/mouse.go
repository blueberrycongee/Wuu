package tui

// focusArea identifies which UI region has keyboard focus.
type focusArea int

const (
	focusInput focusArea = iota
	focusChat
)

// hitTest determines which layout region a mouse click falls in.
func hitTest(x, y int, l layout) focusArea {
	if y >= l.Input.Y && y < l.Input.Y+l.Input.Height+2 {
		return focusInput
	}
	return focusChat
}

// isInFooter returns true if the click is in the footer area.
func isInFooter(y int, l layout) bool {
	return y >= l.Footer.Y
}

// isInHeader returns true if the click is in the header area.
func isInHeader(y int, l layout) bool {
	return y < l.Header.Height
}

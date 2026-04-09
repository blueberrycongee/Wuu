package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	scrollbarThumb         = "┃"
	scrollbarTrack         = "│"
	scrollbarMarker        = "•"
	scrollbarMarkerOnThumb = "●"
)

// renderScrollbar builds a vertical scrollbar string based on content and viewport size.
// Returns an empty string if content fits within the viewport.
func renderScrollbar(height, contentSize, viewportSize, offset int) string {
	return renderScrollbarWithMarkers(height, contentSize, viewportSize, offset, nil)
}

func renderScrollbarWithMarkers(height, contentSize, viewportSize, offset int, markerLines []int) string {
	thumbPos, thumbSize, _, _, ok := scrollbarThumbGeometry(height, contentSize, viewportSize, offset)
	if !ok {
		return ""
	}

	thumbStyle := lipgloss.NewStyle().Foreground(currentTheme.Brand)
	trackStyle := lipgloss.NewStyle().Foreground(currentTheme.Border)
	markerStyle := lipgloss.NewStyle().Foreground(currentTheme.BrandLight)
	markerOnThumbStyle := lipgloss.NewStyle().Foreground(currentTheme.BrandLight)
	markerRows := markerLinesToScrollbarRows(markerLines, height, contentSize)

	var sb strings.Builder
	for i := range height {
		if i > 0 {
			sb.WriteString("\n")
		}
		_, hasMarker := markerRows[i]
		inThumb := i >= thumbPos && i < thumbPos+thumbSize
		switch {
		case inThumb && hasMarker:
			sb.WriteString(markerOnThumbStyle.Render(scrollbarMarkerOnThumb))
		case inThumb:
			sb.WriteString(thumbStyle.Render(scrollbarThumb))
		case hasMarker:
			sb.WriteString(markerStyle.Render(scrollbarMarker))
		default:
			sb.WriteString(trackStyle.Render(scrollbarTrack))
		}
	}

	return sb.String()
}

func scrollbarThumbGeometry(height, contentSize, viewportSize, offset int) (thumbPos, thumbSize, trackSpace, maxOffset int, ok bool) {
	if height <= 0 || viewportSize <= 0 || contentSize <= viewportSize {
		return 0, 0, 0, 0, false
	}
	maxOffset = contentSize - viewportSize
	if maxOffset <= 0 {
		return 0, 0, 0, 0, false
	}
	// Thumb size proportional to visible ratio, minimum 1.
	thumbSize = height * viewportSize / contentSize
	if thumbSize < 1 {
		thumbSize = 1
	} else if thumbSize > height {
		thumbSize = height
	}
	if offset < 0 {
		offset = 0
	} else if offset > maxOffset {
		offset = maxOffset
	}
	trackSpace = height - thumbSize
	if trackSpace > 0 {
		thumbPos = offset * trackSpace / maxOffset
		if thumbPos > trackSpace {
			thumbPos = trackSpace
		}
	}
	return thumbPos, thumbSize, trackSpace, maxOffset, true
}

func markerLinesToScrollbarRows(markerLines []int, height, contentSize int) map[int]struct{} {
	rows := make(map[int]struct{}, len(markerLines))
	for _, row := range contentLinesToScrollbarRows(markerLines, height, contentSize) {
		rows[row] = struct{}{}
	}
	return rows
}

func contentLinesToScrollbarRows(lines []int, height, contentSize int) []int {
	rows := make([]int, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, contentLineToScrollbarRow(line, height, contentSize))
	}
	return rows
}

func contentLineToScrollbarRow(line, height, contentSize int) int {
	if height <= 1 || contentSize <= 1 {
		return 0
	}
	maxLine := contentSize - 1
	if line < 0 {
		line = 0
	} else if line > maxLine {
		line = maxLine
	}
	return line * (height - 1) / maxLine
}

// overlayScrollbar places each scrollbar character at the rightmost column
// of the corresponding viewport line. This avoids shrinking the viewport
// width which would cause content truncation.
func overlayScrollbar(viewport, scrollbar string, totalWidth int) string {
	if totalWidth <= 0 {
		return viewport
	}

	vLines := strings.Split(viewport, "\n")
	sLines := strings.Split(scrollbar, "\n")

	for i := range vLines {
		if i >= len(sLines) {
			break
		}

		// Fit content to one column less than viewport, then place scrollbar in
		// the last column. Padding after truncation is required for wide runes.
		content := fitToWidth(vLines[i], totalWidth-1)
		vLines[i] = content + sLines[i]
	}
	return strings.Join(vLines, "\n")
}

// truncateToWidth cuts a string (possibly with ANSI codes) to a visual width.
func truncateToWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}

// fitToWidth ensures the returned string has exactly the target visual width.
func fitToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	out := truncateToWidth(s, width)
	current := lipgloss.Width(out)
	if current < width {
		out += strings.Repeat(" ", width-current)
	}
	return out
}

# Input Mouse Click Cursor Positioning

## Problem

The input textarea only supports left/right arrow keys for cursor movement. Users expect to click anywhere in the input to position the cursor, as in GUI text editors.

## Design

Add a mouse click handler in `model.go`'s `tea.MouseMsg` case. When a left click lands inside the input area:

1. Compute target row: `msg.Y - inputAreaTop` (accounting for border in non-compact mode)
2. Compute target column: `msg.X - inputAreaLeft - promptWidth` (prompt is `"> "`, 2 cols; border adds 1 col in non-compact mode)
3. Move cursor to target row via `CursorUp()`/`CursorDown()` loops (no `SetRow()` API available)
4. Set column via `SetCursor(targetCol)` — auto-clamps to line bounds

### Edge cases

- Click on prompt area (X too small): move to column 0
- Click past end of line: `SetCursor` clamps to line end
- Click on row beyond line count: go to last line
- Compact vs non-compact border offset handled via `layout.Compact`

### Scope

Single file change: `internal/tui/model.go`, ~20-30 lines in the existing `MouseMsg` branch.

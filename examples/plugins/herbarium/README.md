# 苔径 Herbarium

Herbarium turns the Wuu desktop into a botanical field journal: cream paper,
umber ink, pine-moss accents and serif typography. It ships two coordinated
editions — **苔纸 Herbarium Paper** (light) and **苔夜 Herbarium Night** (dark) —
built entirely on public plugin contracts.

## Try it

1. Install this directory from Wuu's Skills and Plugins catalog
   (or `wuu plugin install .`).
2. Approve and enable **苔径 Herbarium**.
3. Select **苔纸 Herbarium Paper** or **苔夜 Herbarium Night** under
   **Settings → Appearance**.

Disable the plugin (or switch back to a built-in theme) and every rule below
unloads at once: the declarative themes carry all tokens, and the single CSS
snippet is scoped to `:root[data-herbarium]`, which the desktop entry stamps
only while a Herbarium theme is the applied theme. Detection fingerprints the
public tokens the host stamps on `<html>` — no host-private storage keys,
class names, or DOM structure.

## What changes

- **Every color, type, radius, border, shadow and motion value** comes from
  the two declarative themes, including the user-bubble `--wuu-message-user-*`
  surface and a full syntax palette per edition.
- **Left rail**: hairline gutter with a 2px moss spine.
- **Session tabs**: folder index cards; the active tab carries a press rule.
- **Composer**: a letterpress writing card (strong border, debossed inner
  edge, italic placeholder) with a wax-seal send button; the stop state uses
  the clay danger tone.
- **Right panel**: printed masthead (double rule), bookmarked tabs, one
  bordered paper-raised button family across browser, terminal and review
  toolbars.
- **File tree and terminal**: the public shadow-DOM and xterm bridges carry
  the palette into the tree (serif labels, moss selection) and give the
  terminal a deep pine screen in both editions.
- **Settings**: same spine, bookmark-tab navigation (`aria-current`), flat
  press cards.
- **Dialogs, menus, popovers**: bookplate frames (inner hairline outline),
  stronger borders and overlay elevation; tooltips read as italic
  annotations; notices get an accent leading edge.
- **Global**: moss-tinted text selection and a penciled (dashed) focus
  outline.

## Design notes

- Light edition: unbleached-paper ramp (`#f3eee0` canvas), umber ink
  (`#2d2a20`), pine accent (`#3f6b4f`, ≈5.4:1 on canvas).
- Dark edition: green-black ramp (`#1b1f1a` canvas), parchment ink
  (`#e5e0cb`), sage accent (`#92b795`).
- Typography: system serif stack (`ui-serif`/New York/Georgia/Songti SC) at
  the default sizes with a looser 1.62 body leading; code stays monospace.
- Print geometry: 4px controls / 8px panels / 10px overlays, hairline
  borders, near-flat elevation, slightly calmer 220ms motion.
- The terminal is the single deliberate inversion: a dark pine screen even
  in the Paper edition, like ink soaking into the page.

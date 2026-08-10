# Theme surface matrix

This page is generated from the host stylesheet dependency graph by
`scripts/generate-theme-surface-matrix.ts` (run
`make generate-theme-surface-matrix`); do not edit it by hand. The
machine-readable matrix lives at
[`config/desktop-theme-surface-matrix.json`](../../../config/desktop-theme-surface-matrix.json)
and the U1 coverage baseline at
[`themeCoverage.baseline.txt`](../../../desktop/src/renderer/styles/themeCoverage.baseline.txt).

Every row is one `var()` reference of a color-kind paint declaration in the
host stylesheets. A row is **bridged** when a public token reaches the
variable through the custom-property dependency graph, and **unbridged**
otherwise. Geometry-only declarations are excluded.

Current totals: **2111 rows** (1203 bridged,
908 unbridged), **56 public tokens** reach a
host surface, **78 anchors** published.

## Anchor coverage

Host stylesheets currently target component class names: none of the
**78 anchors** has a host CSS selector referencing it, and
**0 of 2111 rows** are attributed to a
`data-wuu-component` anchor. The remaining rows fall into the per-file
`unanchored` bucket until the host migrates selectors to anchors.

| anchor | rows |
| --- | --- |
| `app-shell` | 0 |
| `channel-view` | 0 |
| `composer` | 0 |
| `composer-frame` | 0 |
| `composer-input` | 0 |
| `composer-pending` | 0 |
| `composer-send` | 0 |
| `composer-toolbar` | 0 |
| `conversation-pane` | 0 |
| `conversation-titlebar` | 0 |
| `dialog` | 0 |
| `empty-session` | 0 |
| `environment-panel` | 0 |
| `jump-to-latest` | 0 |
| `launch-view` | 0 |
| `layer-host` | 0 |
| `menu` | 0 |
| `message` | 0 |
| `message-actions` | 0 |
| `message-bubble` | 0 |
| `modal-backdrop` | 0 |
| `notice` | 0 |
| `plugin-contribution` | 0 |
| `plugin-inspector-sections` | 0 |
| `plugin-navigation` | 0 |
| `plugin-navigation-item` | 0 |
| `plugin-settings` | 0 |
| `plugin-settings-navigation` | 0 |
| `plugin-view-content` | 0 |
| `popover` | 0 |
| `session-tab` | 0 |
| `session-tab-close` | 0 |
| `session-tab-main` | 0 |
| `settings-content` | 0 |
| `settings-group` | 0 |
| `settings-navigation` | 0 |
| `settings-navigation-item` | 0 |
| `settings-page` | 0 |
| `settings-row` | 0 |
| `settings-section` | 0 |
| `settings-shell` | 0 |
| `settings-sidebar` | 0 |
| `side-thread` | 0 |
| `sidebar` | 0 |
| `sidebar-toggle` | 0 |
| `skills-catalog` | 0 |
| `tooltip` | 0 |
| `turn` | 0 |
| `workspace-browser` | 0 |
| `workspace-browser-address` | 0 |
| `workspace-browser-content` | 0 |
| `workspace-browser-statusbar` | 0 |
| `workspace-browser-toolbar` | 0 |
| `workspace-document-turn` | 0 |
| `workspace-empty-icon` | 0 |
| `workspace-empty-state` | 0 |
| `workspace-file-content` | 0 |
| `workspace-file-tree` | 0 |
| `workspace-files` | 0 |
| `workspace-panel` | 0 |
| `workspace-panel-header` | 0 |
| `workspace-pdf-preview` | 0 |
| `workspace-review` | 0 |
| `workspace-review-content` | 0 |
| `workspace-review-content-header` | 0 |
| `workspace-review-item` | 0 |
| `workspace-review-navigation` | 0 |
| `workspace-review-search` | 0 |
| `workspace-terminal` | 0 |
| `workspace-terminal-content` | 0 |
| `workspace-terminal-item` | 0 |
| `workspace-terminal-layout` | 0 |
| `workspace-terminal-navigation` | 0 |
| `workspace-terminal-screen` | 0 |
| `workspace-tool` | 0 |
| `workspace-tool-picker` | 0 |
| `workspace-tool-tab` | 0 |
| `workspace-tool-tab-close` | 0 |
| `unanchored:archive-tip.css` (synthetic) | 16 |
| `unanchored:base.css` (synthetic) | 5 |
| `unanchored:channels.css` (synthetic) | 251 |
| `unanchored:chat.css` (synthetic) | 9 |
| `unanchored:composer-context-meter.css` (synthetic) | 10 |
| `unanchored:composer-token-gauge.css` (synthetic) | 7 |
| `unanchored:composer.css` (synthetic) | 182 |
| `unanchored:conversation-shell.css` (synthetic) | 104 |
| `unanchored:environment.css` (synthetic) | 154 |
| `unanchored:image-preview.css` (synthetic) | 11 |
| `unanchored:responsive-design.css` (synthetic) | 32 |
| `unanchored:scrollbars.css` (synthetic) | 6 |
| `unanchored:select-menu.css` (synthetic) | 43 |
| `unanchored:settings.css` (synthetic) | 189 |
| `unanchored:side-thread.css` (synthetic) | 18 |
| `unanchored:sidebar.css` (synthetic) | 131 |
| `unanchored:task-board.css` (synthetic) | 15 |
| `unanchored:task-cards.css` (synthetic) | 11 |
| `unanchored:theme.css` (synthetic) | 6 |
| `unanchored:tooltip.css` (synthetic) | 4 |
| `unanchored:turns.css` (synthetic) | 367 |
| `unanchored:workbench.css` (synthetic) | 96 |
| `unanchored:workspace-pdf-preview.css` (synthetic) | 18 |
| `unanchored:workspace.css` (synthetic) | 426 |

## Tokens and the surfaces they reach

Surfaces are shown as `file · state · property` cells (with the number of
declarations when a cell has more than one). `—` means the token is declared
in the contract but reaches no host paint declaration.

| token | surfaces |
| --- | --- |
| `--hljs-comment` | `workspace.css` · default · `color` |
| `--hljs-function` | `workspace.css` · default · `color` |
| `--hljs-keyword` | `workspace.css` · default · `color` |
| `--hljs-literal` | `workspace.css` · default · `color` |
| `--hljs-meta` | `workspace.css` · default · `color` |
| `--hljs-number` | `workspace.css` · default · `color` |
| `--hljs-string` | `workspace.css` · default · `color` |
| `--hljs-tag` | `workspace.css` · default · `color` |
| `--wuu-accent` | `channels.css` · default · `background`<br>`channels.css` · default · `border-top-color`<br>`channels.css` · default · `box-shadow`<br>`channels.css` · default · `color`<br>`channels.css` · default · `stroke`<br>`composer.css` · default · `background`<br>`conversation-shell.css` · default · `accent-color`<br>`conversation-shell.css` · default · `background` (3)<br>`conversation-shell.css` · default · `border`<br>`sidebar.css` · default · `background`<br>`sidebar.css` · default · `border` (2)<br>`sidebar.css` · default · `color`<br>`workbench.css` · default · `border-top-color`<br>`workspace.css` · default · `background` (3)<br>`workspace.css` · default · `border-left`<br>`workspace.css` · default · `box-shadow` |
| `--wuu-accent-press` | `composer.css` · hover · `background` |
| `--wuu-badge-neutral-background` | `workspace.css` · default · `background` (2) |
| `--wuu-border-strong` | `environment.css` · default · `border` |
| `--wuu-border-subtle` | `sidebar.css` · default · `border-right`<br>`workbench.css` · default · `border`<br>`workspace.css` · default · `border-top` |
| `--wuu-color-accent` | `channels.css` · default · `background`<br>`channels.css` · default · `border-top-color`<br>`channels.css` · default · `box-shadow`<br>`channels.css` · default · `color`<br>`channels.css` · default · `stroke`<br>`composer.css` · default · `background`<br>`conversation-shell.css` · default · `accent-color`<br>`conversation-shell.css` · default · `background` (3)<br>`conversation-shell.css` · default · `border`<br>`sidebar.css` · default · `background`<br>`sidebar.css` · default · `border` (2)<br>`sidebar.css` · default · `color`<br>`workbench.css` · default · `border-top-color` (2)<br>`workspace.css` · default · `background` (3)<br>`workspace.css` · default · `border-left`<br>`workspace.css` · default · `box-shadow` |
| `--wuu-color-accent-pressed` | `composer.css` · hover · `background` |
| `--wuu-color-border-strong` | `channels.css` · default · `background`<br>`channels.css` · default · `border` (4)<br>`channels.css` · default · `border-color`<br>`channels.css` · default · `border-left`<br>`channels.css` · default · `border-top`<br>`channels.css` · default · `stroke`<br>`composer.css` · default · `border-color`<br>`composer.css` · hover · `border-color`<br>`conversation-shell.css` · hover · `border-color`<br>`environment.css` · default · `border-left`<br>`select-menu.css` · default · `border-color` (2)<br>`select-menu.css` · hover · `border-color`<br>`settings.css` · hover · `border-color` (2)<br>`side-thread.css` · default · `border-left`<br>`tooltip.css` · default · `border`<br>`turns.css` · default · `border-bottom-color`<br>`workbench.css` · default · `background`<br>`workbench.css` · default · `border-color` |
| `--wuu-color-border-subtle` | `archive-tip.css` · default · `border`<br>`channels.css` · default · `border` (6)<br>`channels.css` · default · `border-bottom` (6)<br>`channels.css` · default · `border-right` (4)<br>`channels.css` · default · `border-top` (4)<br>`chat.css` · default · `background`<br>`composer.css` · default · `border` (6)<br>`conversation-shell.css` · default · `border` (2)<br>`conversation-shell.css` · default · `border-bottom`<br>`environment.css` · default · `border` (3)<br>`environment.css` · default · `border-top`<br>`select-menu.css` · default · `background`<br>`select-menu.css` · default · `border` (2)<br>`select-menu.css` · default · `border-bottom`<br>`settings.css` · default · `border` (2)<br>`settings.css` · default · `border-bottom`<br>`sidebar.css` · default · `background`<br>`sidebar.css` · default · `border` (2)<br>`task-board.css` · default · `border`<br>`task-board.css` · default · `border-color`<br>`task-cards.css` · default · `border`<br>`turns.css` · default · `background`<br>`turns.css` · default · `border`<br>`turns.css` · default · `border-bottom`<br>`turns.css` · hover · `border-color`<br>`workbench.css` · default · `border` (6)<br>`workbench.css` · default · `border-bottom`<br>`workbench.css` · default · `border-left`<br>`workspace.css` · default · `border` (4)<br>`workspace.css` · default · `border-top` (3) |
| `--wuu-color-canvas` | `archive-tip.css` · default · `background`<br>`channels.css` · default · `background` (19)<br>`channels.css` · default · `border` (3)<br>`channels.css` · default · `box-shadow` (2)<br>`channels.css` · default · `color`<br>`channels.css` · hover · `background`<br>`composer-context-meter.css` · default · `background`<br>`composer-token-gauge.css` · default · `background`<br>`composer-token-gauge.css` · default · `fill`<br>`composer.css` · default · `background` (9)<br>`composer.css` · default · `border`<br>`composer.css` · default · `color`<br>`composer.css` · hover · `color`<br>`conversation-shell.css` · default · `background` (2)<br>`environment.css` · default · `background` (4)<br>`environment.css` · default · `color` (2)<br>`responsive-design.css` · default · `background`<br>`responsive-design.css` · default · `color` (2)<br>`select-menu.css` · default · `background` (2)<br>`select-menu.css` · default · `border`<br>`select-menu.css` · hover · `color`<br>`settings.css` · default · `background` (4)<br>`settings.css` · default · `color`<br>`settings.css` · hover · `background` (2)<br>`side-thread.css` · default · `background`<br>`task-cards.css` · default · `background`<br>`theme.css` · default · `background`<br>`turns.css` · default · `background` (19)<br>`turns.css` · default · `border`<br>`turns.css` · hover · `background`<br>`workbench.css` · default · `background` (10)<br>`workbench.css` · default · `color` (2)<br>`workbench.css` · hover · `background` (2)<br>`workspace-pdf-preview.css` · default · `background` (5)<br>`workspace.css` · default · `background` (27)<br>`workspace.css` · default · `background-color`<br>`workspace.css` · default · `color` |
| `--wuu-color-danger` | `archive-tip.css` · default · `color`<br>`base.css` · default · `color`<br>`channels.css` · default · `background`<br>`channels.css` · default · `border`<br>`channels.css` · default · `color` (4)<br>`channels.css` · hover · `color` (2)<br>`composer.css` · default · `color` (3)<br>`composer.css` · hover · `color`<br>`conversation-shell.css` · default · `color` (2)<br>`environment.css` · default · `border`<br>`environment.css` · default · `color` (5)<br>`settings.css` · default · `background`<br>`settings.css` · default · `color` (5)<br>`sidebar.css` · default · `color` (2)<br>`task-board.css` · default · `color`<br>`task-cards.css` · default · `color`<br>`turns.css` · default · `background`<br>`turns.css` · default · `color` (16)<br>`turns.css` · hover · `color`<br>`workbench.css` · default · `color` (4)<br>`workbench.css` · hover · `background` (2)<br>`workspace-pdf-preview.css` · default · `color`<br>`workspace.css` · default · `background`<br>`workspace.css` · default · `color` (14)<br>`workspace.css` · hover · `color` |
| `--wuu-color-focus` | `channels.css` · default · `outline`<br>`composer-context-meter.css` · default · `outline`<br>`composer-token-gauge.css` · default · `outline`<br>`composer.css` · default · `outline`<br>`conversation-shell.css` · default · `box-shadow`<br>`turns.css` · default · `outline` |
| `--wuu-color-info` | `archive-tip.css` · default · `color`<br>`archive-tip.css` · default · `outline` (2)<br>`archive-tip.css` · hover · `color`<br>`channels.css` · default · `background` (2)<br>`channels.css` · hover · `color`<br>`composer.css` · hover · `border-color`<br>`composer.css` · hover · `color`<br>`environment.css` · default · `accent-color`<br>`environment.css` · default · `border`<br>`environment.css` · default · `border-color`<br>`environment.css` · default · `outline`<br>`environment.css` · hover · `border-color`<br>`sidebar.css` · default · `background` (3)<br>`task-cards.css` · default · `color` (2)<br>`turns.css` · default · `accent-color`<br>`turns.css` · default · `color`<br>`workspace.css` · default · `color` (2) |
| `--wuu-color-link` | `channels.css` · default · `color`<br>`channels.css` · default · `outline`<br>`turns.css` · default · `color` (2)<br>`turns.css` · default · `outline`<br>`turns.css` · hover · `color`<br>`workspace.css` · default · `color` (2) |
| `--wuu-color-link-hover` | `channels.css` · hover · `color`<br>`turns.css` · hover · `color` (2)<br>`workspace.css` · hover · `color` (2) |
| `--wuu-color-live-highlight` | `base.css` · default · `background-image` |
| `--wuu-color-on-accent` | `composer.css` · default · `color`<br>`sidebar.css` · default · `color` |
| `--wuu-color-overlay-backdrop` | `base.css` · default · `background`<br>`workspace.css` · default · `background` |
| `--wuu-color-success` | `channels.css` · default · `background` (2)<br>`channels.css` · default · `border-top-color`<br>`channels.css` · default · `color` (2)<br>`channels.css` · default · `fill`<br>`conversation-shell.css` · default · `color` (2)<br>`environment.css` · default · `color`<br>`settings.css` · default · `background`<br>`settings.css` · default · `color` (2)<br>`sidebar.css` · default · `color`<br>`task-cards.css` · default · `color`<br>`turns.css` · default · `background` (3)<br>`turns.css` · default · `color` (2)<br>`turns.css` · hover · `color`<br>`workspace.css` · default · `color` (4) |
| `--wuu-color-surface` | `channels.css` · default · `background`<br>`composer.css` · default · `background` (2)<br>`composer.css` · hover · `background`<br>`conversation-shell.css` · default · `background`<br>`environment.css` · hover · `background`<br>`select-menu.css` · default · `background`<br>`settings.css` · default · `background` (7)<br>`turns.css` · default · `background` (11)<br>`turns.css` · hover · `background` (2)<br>`workbench.css` · default · `background` (2)<br>`workspace.css` · default · `background` (5) |
| `--wuu-color-surface-elevated` | `channels.css` · default · `background`<br>`channels.css` · hover · `background` (2)<br>`composer.css` · default · `background` (7)<br>`composer.css` · default · `border`<br>`composer.css` · hover · `background`<br>`conversation-shell.css` · default · `background` (8)<br>`conversation-shell.css` · default · `border`<br>`conversation-shell.css` · default · `border-bottom` (2)<br>`conversation-shell.css` · hover · `background` (3)<br>`environment.css` · default · `background` (7)<br>`environment.css` · default · `border-bottom`<br>`environment.css` · default · `border-top` (2)<br>`select-menu.css` · default · `background` (2)<br>`settings.css` · default · `background` (2)<br>`settings.css` · hover · `background`<br>`side-thread.css` · default · `border-bottom`<br>`side-thread.css` · default · `border-top`<br>`side-thread.css` · hover · `border-color`<br>`sidebar.css` · default · `background` (2)<br>`sidebar.css` · default · `background-color` (2)<br>`tooltip.css` · default · `background`<br>`turns.css` · default · `background` (2)<br>`turns.css` · default · `border` (4)<br>`turns.css` · default · `border-bottom` (3)<br>`turns.css` · default · `border-top`<br>`turns.css` · hover · `background` (3)<br>`workbench.css` · default · `background` (2)<br>`workbench.css` · hover · `background`<br>`workspace-pdf-preview.css` · default · `border`<br>`workspace-pdf-preview.css` · default · `border-bottom`<br>`workspace.css` · default · `background` (12)<br>`workspace.css` · default · `border` (2)<br>`workspace.css` · default · `border-bottom` (9)<br>`workspace.css` · default · `border-left`<br>`workspace.css` · default · `border-left-color`<br>`workspace.css` · default · `border-right` (2)<br>`workspace.css` · default · `border-top` (2)<br>`workspace.css` · default · `border-top-color`<br>`workspace.css` · hover · `background` (8) |
| `--wuu-color-surface-muted` | `channels.css` · default · `background` (7)<br>`channels.css` · hover · `background` (5)<br>`channels.css` · selected · `background`<br>`composer.css` · default · `background` (2)<br>`composer.css` · disabled · `background`<br>`composer.css` · hover · `background` (11)<br>`environment.css` · default · `background` (15)<br>`environment.css` · hover · `background` (6)<br>`environment.css` · selected · `background`<br>`select-menu.css` · default · `background` (2)<br>`select-menu.css` · hover · `background` (2)<br>`settings.css` · default · `background` (8)<br>`settings.css` · hover · `background` (4)<br>`side-thread.css` · default · `background`<br>`side-thread.css` · hover · `background`<br>`sidebar.css` · default · `background` (4)<br>`sidebar.css` · hover · `background` (2)<br>`theme.css` · default · `background`<br>`turns.css` · default · `background` (7)<br>`turns.css` · default · `border-bottom` (2)<br>`turns.css` · default · `border-top`<br>`turns.css` · hover · `background` (3)<br>`workbench.css` · default · `background`<br>`workbench.css` · hover · `background` (4)<br>`workspace-pdf-preview.css` · hover · `background`<br>`workspace.css` · default · `background` (11)<br>`workspace.css` · default · `border-bottom` (2)<br>`workspace.css` · hover · `background` (6) |
| `--wuu-color-text` | `archive-tip.css` · default · `color` (3)<br>`archive-tip.css` · hover · `color`<br>`base.css` · default · `color`<br>`channels.css` · default · `accent-color`<br>`channels.css` · default · `background`<br>`channels.css` · default · `border-color`<br>`channels.css` · default · `box-shadow`<br>`channels.css` · default · `color` (30)<br>`channels.css` · hover · `color` (10)<br>`chat.css` · default · `color`<br>`chat.css` · hover · `color`<br>`composer-context-meter.css` · default · `color` (3)<br>`composer-token-gauge.css` · default · `color`<br>`composer.css` · default · `background` (2)<br>`composer.css` · default · `border`<br>`composer.css` · default · `border-color` (2)<br>`composer.css` · default · `box-shadow` (2)<br>`composer.css` · default · `color` (23)<br>`composer.css` · hover · `background`<br>`composer.css` · hover · `border-color`<br>`composer.css` · hover · `box-shadow`<br>`composer.css` · hover · `color` (10)<br>`composer.css` · selected · `color`<br>`conversation-shell.css` · default · `background` (3)<br>`conversation-shell.css` · default · `border`<br>`conversation-shell.css` · default · `color` (16)<br>`conversation-shell.css` · hover · `background`<br>`conversation-shell.css` · hover · `border-color`<br>`conversation-shell.css` · hover · `color` (4)<br>`environment.css` · default · `background` (2)<br>`environment.css` · default · `color` (29)<br>`environment.css` · hover · `background` (2)<br>`environment.css` · hover · `color` (2)<br>`responsive-design.css` · default · `background` (4)<br>`responsive-design.css` · default · `color` (5)<br>`responsive-design.css` · hover · `color`<br>`select-menu.css` · default · `background` (4)<br>`select-menu.css` · default · `color` (4)<br>`select-menu.css` · hover · `background`<br>`settings.css` · default · `color` (18)<br>`settings.css` · hover · `color` (6)<br>`side-thread.css` · default · `color` (3)<br>`side-thread.css` · hover · `color`<br>`sidebar.css` · default · `color` (10)<br>`sidebar.css` · hover · `color` (9)<br>`task-cards.css` · default · `color` (2)<br>`theme.css` · default · `color`<br>`theme.css` · hover · `color`<br>`tooltip.css` · default · `color`<br>`turns.css` · default · `background` (2)<br>`turns.css` · default · `border`<br>`turns.css` · default · `border-color`<br>`turns.css` · default · `box-shadow` (2)<br>`turns.css` · default · `color` (41)<br>`turns.css` · hover · `color` (9)<br>`workbench.css` · default · `background` (2)<br>`workbench.css` · default · `color` (17)<br>`workbench.css` · hover · `background`<br>`workbench.css` · hover · `color` (4)<br>`workspace-pdf-preview.css` · default · `color` (2)<br>`workspace-pdf-preview.css` · hover · `color`<br>`workspace.css` · default · `background` (2)<br>`workspace.css` · default · `border` (2)<br>`workspace.css` · default · `border-color` (2)<br>`workspace.css` · default · `box-shadow` (3)<br>`workspace.css` · default · `color` (50)<br>`workspace.css` · hover · `border-color`<br>`workspace.css` · hover · `box-shadow`<br>`workspace.css` · hover · `color` (12) |
| `--wuu-color-text-muted` | `archive-tip.css` · default · `color`<br>`channels.css` · default · `color` (25)<br>`channels.css` · default · `fill`<br>`channels.css` · default · `stroke`<br>`composer.css` · default · `color`<br>`conversation-shell.css` · default · `color`<br>`environment.css` · default · `color` (7)<br>`select-menu.css` · default · `color`<br>`settings.css` · default · `color` (8)<br>`task-cards.css` · default · `color`<br>`theme.css` · default · `color`<br>`turns.css` · default · `color` (11)<br>`workbench.css` · default · `color` (7)<br>`workspace.css` · default · `color` (16) |
| `--wuu-color-warning` | `channels.css` · default · `background` (2)<br>`channels.css` · default · `border-top-color`<br>`channels.css` · default · `fill`<br>`composer.css` · default · `border-color`<br>`composer.css` · default · `color` (6)<br>`conversation-shell.css` · default · `color`<br>`side-thread.css` · default · `color`<br>`task-board.css` · default · `background`<br>`task-board.css` · default · `border-color`<br>`task-board.css` · default · `color` (2)<br>`turns.css` · default · `color` (2)<br>`workspace.css` · default · `border`<br>`workspace.css` · default · `color` (3) |
| `--wuu-content-max-width` | — (no host surface) |
| `--wuu-control-field-background` | `select-menu.css` · default · `background`<br>`select-menu.css` · hover · `background`<br>`settings.css` · default · `background` (3)<br>`settings.css` · hover · `background` (3)<br>`workbench.css` · default · `background` (2)<br>`workbench.css` · hover · `background`<br>`workspace.css` · default · `background` |
| `--wuu-control-icon-background` | `environment.css` · default · `background` (2) |
| `--wuu-control-secondary-background` | `environment.css` · default · `background`<br>`responsive-design.css` · default · `background`<br>`settings.css` · default · `background`<br>`settings.css` · hover · `background`<br>`workbench.css` · default · `background`<br>`workbench.css` · hover · `background`<br>`workspace.css` · hover · `background` |
| `--wuu-elevation-overlay` | `archive-tip.css` · default · `box-shadow`<br>`channels.css` · default · `box-shadow` (2)<br>`composer.css` · default · `box-shadow` (4)<br>`conversation-shell.css` · default · `box-shadow`<br>`environment.css` · default · `box-shadow` (2)<br>`select-menu.css` · default · `box-shadow`<br>`sidebar.css` · default · `box-shadow` (2)<br>`tooltip.css` · default · `box-shadow`<br>`turns.css` · default · `box-shadow`<br>`workbench.css` · default · `box-shadow`<br>`workspace.css` · default · `box-shadow` (4) |
| `--wuu-elevation-panel` | `sidebar.css` · default · `box-shadow`<br>`workbench.css` · default · `box-shadow`<br>`workspace.css` · default · `box-shadow` |
| `--wuu-font-family-mono` | — (no host surface) |
| `--wuu-font-family-ui` | — (no host surface) |
| `--wuu-font-size-body` | — (no host surface) |
| `--wuu-font-size-ui` | — (no host surface) |
| `--wuu-hairline` | `chat.css` · default · `background`<br>`turns.css` · default · `background` |
| `--wuu-ink` | — (no host surface) |
| `--wuu-ink-soft` | `turns.css` · default · `color` (2) |
| `--wuu-inline-code-background` | `turns.css` · default · `background`<br>`workspace.css` · default · `background` |
| `--wuu-line-height-body` | — (no host surface) |
| `--wuu-message-action-radius` | — (no host surface) |
| `--wuu-message-action-size` | — (no host surface) |
| `--wuu-message-actions-block-gap` | — (no host surface) |
| `--wuu-message-actions-control-gap` | — (no host surface) |
| `--wuu-message-actions-inline-offset` | — (no host surface) |
| `--wuu-message-actions-overlay-gap` | — (no host surface) |
| `--wuu-message-user-background` | `chat.css` · default · `background`<br>`environment.css` · default · `background`<br>`turns.css` · default · `background` (3) |
| `--wuu-message-user-border` | `turns.css` · default · `border` |
| `--wuu-message-user-color` | `turns.css` · default · `color` |
| `--wuu-message-user-radius` | — (no host surface) |
| `--wuu-message-user-shadow` | `turns.css` · default · `box-shadow` |
| `--wuu-motion-duration-fast` | — (no host surface) |
| `--wuu-motion-duration-normal` | — (no host surface) |
| `--wuu-motion-easing-standard` | — (no host surface) |
| `--wuu-nav-item-hover-background` | `settings.css` · hover · `background` (2)<br>`sidebar.css` · hover · `background` (2) |
| `--wuu-nav-item-hover-ring` | `sidebar.css` · hover · `box-shadow` |
| `--wuu-paper` | `composer-token-gauge.css` · default · `fill` |
| `--wuu-radius-control` | — (no host surface) |
| `--wuu-radius-overlay` | — (no host surface) |
| `--wuu-radius-panel` | — (no host surface) |
| `--wuu-space-density` | — (no host surface) |
| `--wuu-space-unit` | — (no host surface) |
| `--wuu-surface-muted` | — (no host surface) |
| `--wuu-syntax-comment` | `workspace.css` · default · `color` |
| `--wuu-syntax-function` | `workspace.css` · default · `color` |
| `--wuu-syntax-keyword` | `workspace.css` · default · `color` |
| `--wuu-syntax-literal` | `workspace.css` · default · `color` |
| `--wuu-syntax-meta` | `workspace.css` · default · `color` |
| `--wuu-syntax-number` | `workspace.css` · default · `color` |
| `--wuu-syntax-string` | `workspace.css` · default · `color` |
| `--wuu-syntax-tag` | `workspace.css` · default · `color` |

## Unbridged surfaces

Unbridged rows are the U1 baseline entries: unanchored per file, priority
order workspace → turns → sidebar → settings → composer/conversation-shell →
channels → environment → image-preview → rest. Current distribution:
turns.css 192, workspace.css 184, settings.css 104, sidebar.css 81, channels.css 76, composer.css 73, environment.css 49, conversation-shell.css 47, responsive-design.css 18, workbench.css 15, select-menu.css 12, image-preview.css 11, task-board.css 8, scrollbars.css 6, side-thread.css 6, workspace-pdf-preview.css 6, chat.css 5, composer-context-meter.css 5, archive-tip.css 3, composer-token-gauge.css 3, task-cards.css 2, base.css 1, theme.css 1.

The only sanctioned bridge form is a single declaration:

```css
prop: var(--wuu-slot, var(--private-fallback));
```

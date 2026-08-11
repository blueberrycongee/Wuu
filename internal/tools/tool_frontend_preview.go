package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	frontendPreviewToolName        = "render_frontend_preview"
	frontendPreviewVersion         = 1
	frontendPreviewMaxPayloadBytes = 256 * 1024
	frontendPreviewMaxTitleBytes   = 80
	frontendPreviewMaxHTMLBytes    = 96 * 1024
	frontendPreviewMaxCSSBytes     = 96 * 1024
	frontendPreviewMaxJSBytes      = 96 * 1024
	frontendPreviewMaxHTMLNodes    = 500
	frontendPreviewDefaultHeight   = 320
	frontendPreviewMinHeight       = 160
	frontendPreviewMaxHeight       = 720
)

type frontendPreviewViewport struct {
	Height int `json:"height,omitempty"`
}

type frontendPreviewSpec struct {
	Version    int                     `json:"version"`
	Title      string                  `json:"title"`
	HTML       string                  `json:"html"`
	CSS        string                  `json:"css,omitempty"`
	JavaScript string                  `json:"javascript,omitempty"`
	Viewport   frontendPreviewViewport `json:"viewport,omitempty"`
}

// FrontendPreviewTool validates and records an immutable HTML/CSS/JavaScript
// snapshot. Rendering belongs to a shell that explicitly negotiated support;
// the Go core never evaluates model-generated frontend code.
type FrontendPreviewTool struct{}

func NewFrontendPreviewTool() *FrontendPreviewTool { return &FrontendPreviewTool{} }

func (t *FrontendPreviewTool) Name() string { return frontendPreviewToolName }

func (t *FrontendPreviewTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name: frontendPreviewToolName,
		Description: "Render a self-contained HTML, CSS, and JavaScript prototype as a collapsed interactive card inside the current desktop conversation turn. " +
			"Use this for small frontend studies such as buttons, cards, forms, menus, toasts, and animations. The preview has no network, filesystem, Electron, Node.js, package installation, or workspace-project access. " +
			"Provide a short textual conclusion after the tool call.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"version": map[string]any{
					"type":        "integer",
					"const":       frontendPreviewVersion,
					"description": "Required schema version. Use 1.",
				},
				"title": map[string]any{
					"type":      "string",
					"minLength": 1,
					"maxLength": frontendPreviewMaxTitleBytes,
				},
				"html": map[string]any{
					"type":        "string",
					"description": "An HTML body fragment. Do not include html, head, body, script, style, iframe, link, meta, or external-resource elements.",
				},
				"css": map[string]any{
					"type":        "string",
					"description": "Optional self-contained CSS. External imports and url() resources are not allowed.",
				},
				"javascript": map[string]any{
					"type":        "string",
					"description": "Optional browser JavaScript. Network, navigation, popup, worker, Electron, Node.js, and host-document access are unavailable.",
				},
				"viewport": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"height": map[string]any{
							"type":    "integer",
							"minimum": frontendPreviewMinHeight,
							"maximum": frontendPreviewMaxHeight,
						},
					},
				},
			},
			"required": []string{"version", "title", "html"},
		},
	}
}

func (t *FrontendPreviewTool) IsReadOnly() bool        { return true }
func (t *FrontendPreviewTool) IsConcurrencySafe() bool { return true }

func (t *FrontendPreviewTool) ValidateInput(argsJSON string) error {
	_, err := parseFrontendPreviewSpec(argsJSON)
	return err
}

func (t *FrontendPreviewTool) Execute(_ context.Context, argsJSON string) (string, error) {
	result, err := t.ExecuteResult(context.Background(), argsJSON)
	return result.TextProjection(), err
}

func (t *FrontendPreviewTool) ExecuteResult(_ context.Context, argsJSON string) (toolresult.Result, error) {
	if _, err := parseFrontendPreviewSpec(argsJSON); err != nil {
		return toolresult.Result{}, err
	}
	digestBytes := sha256.Sum256([]byte(argsJSON))
	meta, err := json.Marshal(map[string]any{
		"presentation": map[string]any{
			"kind":    "frontend_preview",
			"version": frontendPreviewVersion,
			"digest":  "sha256:" + hex.EncodeToString(digestBytes[:]),
		},
	})
	if err != nil {
		return toolresult.Result{}, fmt.Errorf("encode frontend preview acknowledgement: %w", err)
	}
	return toolresult.Result{
		Content: []toolresult.ContentPart{{Type: toolresult.ContentTypeText, Text: "Frontend preview ready."}},
		Meta:    meta,
	}, nil
}

func parseFrontendPreviewSpec(argsJSON string) (frontendPreviewSpec, error) {
	var spec frontendPreviewSpec
	if len(argsJSON) > frontendPreviewMaxPayloadBytes {
		return spec, fmt.Errorf("frontend preview payload exceeds %d bytes: error_kind=payload_too_large", frontendPreviewMaxPayloadBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(argsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return spec, fmt.Errorf("invalid frontend preview arguments: %w", err)
	}
	if err := ensureFrontendPreviewJSONEOF(decoder); err != nil {
		return spec, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argsJSON), &fields); err != nil {
		return spec, fmt.Errorf("invalid frontend preview arguments: %w", err)
	}
	if _, ok := fields["html"]; !ok {
		return spec, errors.New("frontend preview html is required: error_kind=missing_html")
	}
	if spec.Version != frontendPreviewVersion {
		return spec, fmt.Errorf("unsupported frontend preview version %d: error_kind=unsupported_version", spec.Version)
	}
	if title := strings.TrimSpace(spec.Title); title == "" {
		return spec, errors.New("frontend preview title is required: error_kind=missing_title")
	} else if len(title) > frontendPreviewMaxTitleBytes {
		return spec, fmt.Errorf("frontend preview title exceeds %d bytes: error_kind=title_too_large", frontendPreviewMaxTitleBytes)
	}
	if len(spec.HTML) > frontendPreviewMaxHTMLBytes {
		return spec, fmt.Errorf("frontend preview html exceeds %d bytes: error_kind=html_too_large", frontendPreviewMaxHTMLBytes)
	}
	if len(spec.CSS) > frontendPreviewMaxCSSBytes {
		return spec, fmt.Errorf("frontend preview css exceeds %d bytes: error_kind=css_too_large", frontendPreviewMaxCSSBytes)
	}
	if len(spec.JavaScript) > frontendPreviewMaxJSBytes {
		return spec, fmt.Errorf("frontend preview javascript exceeds %d bytes: error_kind=javascript_too_large", frontendPreviewMaxJSBytes)
	}
	if spec.Viewport.Height == 0 {
		spec.Viewport.Height = frontendPreviewDefaultHeight
	}
	if spec.Viewport.Height < frontendPreviewMinHeight || spec.Viewport.Height > frontendPreviewMaxHeight {
		return spec, fmt.Errorf("frontend preview height must be between %d and %d: error_kind=invalid_viewport", frontendPreviewMinHeight, frontendPreviewMaxHeight)
	}
	if err := validateFrontendPreviewHTML(spec.HTML); err != nil {
		return spec, err
	}
	if err := validateFrontendPreviewCSS(spec.CSS); err != nil {
		return spec, err
	}
	return spec, nil
}

func ensureFrontendPreviewJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("invalid frontend preview arguments: %w", err)
	}
	return errors.New("invalid frontend preview arguments: trailing JSON value")
}

var frontendPreviewForbiddenElements = map[string]struct{}{
	"applet": {}, "base": {}, "body": {}, "embed": {}, "frame": {}, "frameset": {},
	"head": {}, "html": {}, "iframe": {}, "link": {}, "meta": {}, "object": {},
	"script": {}, "style": {}, "title": {}, "webview": {},
}

var frontendPreviewForbiddenAttributes = map[string]struct{}{
	"action": {}, "background": {}, "cite": {}, "data": {}, "formaction": {},
	"href": {}, "longdesc": {}, "manifest": {}, "ping": {}, "poster": {},
	"profile": {}, "src": {}, "srcdoc": {}, "srcset": {}, "usemap": {},
}

func validateFrontendPreviewHTML(fragment string) error {
	contextNode := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), contextNode)
	if err != nil {
		return fmt.Errorf("invalid frontend preview html: %w", err)
	}
	count := 0
	var visit func(*html.Node) error
	visit = func(node *html.Node) error {
		count++
		if count > frontendPreviewMaxHTMLNodes {
			return fmt.Errorf("frontend preview html exceeds %d nodes: error_kind=too_many_nodes", frontendPreviewMaxHTMLNodes)
		}
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if _, forbidden := frontendPreviewForbiddenElements[tag]; forbidden {
				return fmt.Errorf("frontend preview html element <%s> is not allowed: error_kind=unsafe_html", tag)
			}
			for _, attr := range node.Attr {
				name := strings.ToLower(attr.Key)
				if strings.HasPrefix(name, "on") {
					return fmt.Errorf("frontend preview html event attribute %q is not allowed: error_kind=unsafe_html", name)
				}
				if _, forbidden := frontendPreviewForbiddenAttributes[name]; forbidden {
					return fmt.Errorf("frontend preview html attribute %q is not allowed: error_kind=unsafe_html", name)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, node := range nodes {
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}

func validateFrontendPreviewCSS(css string) error {
	normalized := strings.ToLower(css)
	for _, forbidden := range []string{"@import", "url(", "expression(", "-moz-binding"} {
		if strings.Contains(normalized, forbidden) {
			return fmt.Errorf("frontend preview css contains forbidden %q: error_kind=unsafe_css", forbidden)
		}
	}
	return nil
}

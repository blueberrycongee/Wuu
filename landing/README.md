# Wuu website

The marketing site is plain HTML and CSS. `index.html` is the product homepage. Unpublished brand-page drafts live in `drafts/` as text files and are excluded from the site build. A small progressive enhancement animates the mascot eyes. Content and navigation work without JavaScript; no external fonts are requested.

Preview from the repository root:

```sh
python3 -m http.server 4173 --bind 127.0.0.1 --directory landing
```

The documentation site renders these same marketing sources. Its preparation script copies only the current brand assets, provider logos and stylesheet. Validate the complete hosted output with:

```sh
CI=true npm --prefix docs-site run build
```

The homepage uses an explicitly labelled workflow illustration, not a captured running session. Download links lead to GitHub Releases so version and packaging details remain current. Desktop application UI is outside this site's scope.

The site includes bilingual product and blog pages. Marketing HTML files are automatically exposed as matching directory routes by the documentation build. The blog currently displays a placeholder. Unpublished articles and their illustrations live in the Git-ignored `.private-drafts/` directory and are excluded from the documentation build. Preview them through the local server at `/.private-drafts/<filename>.html`. Move finished articles into this directory and add their public links when ready to publish. Product, blog, documentation, and download navigation expand on hover, click, or keyboard activation.

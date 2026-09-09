# Wuu website

The marketing site is plain HTML and CSS. `index.html` is the product homepage. Unpublished brand-page drafts live in `drafts/` as text files and are excluded from the site build. A small progressive enhancement animates the mascot: it follows the pointer, hops and wiggles when left alone, and the homepage ball splits into a lingering trio on double-click — or on its own when bored — and merges back the same way. Content and navigation work without JavaScript; no external fonts are requested.

Preview from the repository root:

```sh
python3 -m http.server 4173 --bind 127.0.0.1 --directory landing
```

The documentation site renders these same marketing sources. Its preparation script copies the current brand assets, provider logos, blog images and stylesheets. Validate the complete hosted output with:

```sh
CI=true npm --prefix docs-site run build
```

The homepage uses an explicitly labelled workflow illustration, not a captured running session. Download links lead to GitHub Releases so version and packaging details remain current. Desktop application UI is outside this site's scope.

The site includes bilingual product and blog pages. Marketing HTML files are automatically exposed as matching directory routes by the documentation build. The blog page lists the published articles. Unpublished articles and their illustrations live in the Git-ignored `.private-drafts/` directory and are excluded from the documentation build. Preview them through the local server at `/.private-drafts/<filename>.html`. When an article is ready, move it to the landing root with its images under `assets/blog/`, strip the `.private-drafts/` path prefixes, and link it from the blog page and the site navigation. Product, blog, documentation, and download navigation expand on hover, click, or keyboard activation.

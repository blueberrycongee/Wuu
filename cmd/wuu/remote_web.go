package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// Compress public web assets before they cross a potentially slow relay link.
// Keep the standard file server for ranges, directories, and non-text assets.
func remoteWebHandler(root string) http.Handler {
	fs := http.Dir(root)
	fallback := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		name := path.Clean("/" + r.URL.Path)
		if name == "/" {
			name = "/index.html"
		}
		ext := path.Ext(name)
		text := ext == ".js" || ext == ".css" || ext == ".html" || ext == ".svg" || ext == ".json"
		if (r.Method != "GET" && r.Method != "HEAD") || r.Header.Get("Range") != "" || !text || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			fallback.ServeHTTP(w, r)
			return
		}
		f, err := fs.Open(name)
		if err != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() || info.Size() < 1024 || info.Size() > 16<<20 {
			fallback.ServeHTTP(w, r)
			return
		}
		var compressed bytes.Buffer
		z := gzip.NewWriter(&compressed)
		_, err = io.Copy(z, io.LimitReader(f, (16<<20)+1))
		closeErr := z.Close()
		if err != nil || closeErr != nil {
			http.Error(w, "cannot read web asset", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, r, info.Name(), info.ModTime(), bytes.NewReader(compressed.Bytes()))
	})
}

func acceptsGzip(value string) bool {
	wildcard := false
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, param := range parts[1:] {
			key, val, ok := strings.Cut(strings.TrimSpace(param), "=")
			if ok && strings.EqualFold(key, "q") {
				quality, _ = strconv.ParseFloat(val, 64)
			}
		}
		if coding == "gzip" {
			return quality > 0 && quality <= 1
		}
		if coding == "*" {
			wildcard = quality > 0 && quality <= 1
		}
	}
	return wildcard
}

package main

import (
	"compress/gzip"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteWebCompressedAsset(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("export const workspace = 'ready';\n", 1000)
	if err := os.WriteFile(filepath.Join(root, "workbench.js"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	h := remoteWebHandler(root)
	get := func(method, accept, byteRange string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/workbench.js", nil)
		r.Header.Set("Accept-Encoding", accept)
		r.Header.Set("Range", byteRange)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := get("GET", "br, gzip", "")
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "gzip" || w.Body.Len() >= len(body)/2 {
		t.Fatalf("not compressed: %d %v bytes=%d", w.Code, w.Header(), w.Body.Len())
	}
	z, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(z)
	if err != nil || string(decoded) != body {
		t.Fatal("compressed body did not round trip", err)
	}
	for _, accept := range []string{"", "gzip;q=0, *;q=1", "br"} {
		w = get("GET", accept, "")
		if w.Header().Get("Content-Encoding") != "" || w.Body.String() != body {
			t.Fatalf("invalid fallback for %q", accept)
		}
	}
	w = get("GET", "gzip", "bytes=0-5")
	if w.Code != 206 || w.Body.String() != body[:6] {
		t.Fatalf("range failed: %d %q", w.Code, w.Body.String())
	}
	w = get("HEAD", "gzip", "")
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("invalid HEAD response", w)
	}
	r := httptest.NewRequest("GET", "/workbench.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	r.Header.Set("If-Modified-Since", w.Header().Get("Last-Modified"))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal("conditional request failed", w.Code)
	}
}

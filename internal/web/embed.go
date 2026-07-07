// Package web embeds the built Nuxt SPA (internal/web/dist, produced by
// `task ui:generate`) and serves it as a static single-page app inside the same
// Go binary that serves the JSON API. In production there is no proxy: the API
// is mounted at /api and this handler serves everything else on the same origin,
// so the SPA's same-origin `/api` calls just work.
//
// The dist/ directory is a build artifact. Only a placeholder index.html is
// committed (so this package compiles before a UI build); `task ui:generate`
// overwrites it with the real bundle. `all:dist` is required — a plain `dist`
// pattern would skip Nuxt's `_nuxt/` assets (go:embed ignores `_`-prefixed
// names without the all: prefix).
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the embedded SPA. A request for a real embedded asset is served
// directly; any other path falls back to index.html so the client-side router
// (e.g. /<draftId>) works on a hard refresh or deep link. Every response carries
// the strict, self-only Content-Security-Policy the SPA was built for.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at build time; Sub cannot fail here.
	}
	fileServer := http.FileServer(http.FS(sub))
	index := mustRead(sub, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCSP(w)
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && assetExists(sub, name) {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: unknown paths render the app shell, not a 404.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func assetExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.IsDir() {
		return false // directories fall through to the SPA shell
	}
	return true
}

func mustRead(fsys fs.FS, name string) []byte {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(err) // index.html is always present in dist (placeholder or built).
	}
	return b
}

// setCSP applies a strict Content-Security-Policy whose load-bearing guarantee is
// "no third-party host": default-src/connect-src 'self' block every external
// request. 'unsafe-inline' is allowed for script and style because Nuxt SSG emits
// an inline color-mode/config bootstrap script and Nuxt UI (Tailwind) + Mermaid
// emit inline styles; these are same-document, not third-party. img/font allow
// data: URIs (bundled icons, Mermaid SVG).
func setCSP(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' 'unsafe-inline'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; "+
			"font-src 'self' data:; "+
			"connect-src 'self'; "+
			"object-src 'none'; "+
			"base-uri 'self'; "+
			"frame-ancestors 'none'")
}

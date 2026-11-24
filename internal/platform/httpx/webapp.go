package httpx

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// WebApp serves the built PWA (web/dist) from the same origin as the API, so
// one Cloudflare Access application and one sign-in cover both (ADR-0018).
//
//   - Files under assets/ have content hashes in their names: cached for a year.
//   - Everything else (index.html, the service worker, the manifest) must be
//     revalidated, so a new deploy reaches the phone on its next launch.
//   - A path with no file and no extension is a client-side route: it gets
//     index.html. A missing file with an extension is a 404, so a broken asset
//     link fails loudly instead of returning HTML.
func WebApp(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			WriteProblem(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(files, name)
		if err != nil || info.IsDir() {
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		// Serve the resolved, cleaned name, never the raw path.
		r2 := r.Clone(r.Context())
		if name == "index.html" {
			// http.FileServer redirects /index.html to /; serve the bytes directly.
			r2.URL.Path = "/"
			http.ServeFileFS(w, r2, files, name)
			return
		}
		r2.URL.Path = "/" + name
		fileServer.ServeHTTP(w, r2)
	})
}

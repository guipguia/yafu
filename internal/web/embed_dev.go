//go:build !embed

package web

import "net/http"

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(devIndex))
	})
}

const devIndex = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>yafu</title>
<style>
body{font-family:system-ui,sans-serif;max-width:48rem;margin:4rem auto;padding:0 1rem;color:#1f2937;line-height:1.6}
code{background:#f3f4f6;padding:0.125rem 0.375rem;border-radius:0.25rem;font-size:0.9em}
h1{margin-bottom:0.25rem}
.tag{display:inline-block;background:#fef3c7;color:#92400e;padding:0.125rem 0.5rem;border-radius:0.25rem;font-size:0.75rem;font-weight:600;text-transform:uppercase;letter-spacing:0.05em}
</style>
</head>
<body>
<h1>yafu <span class="tag">dev mode</span></h1>
<p>The Go binary was built without the embedded frontend.</p>
<ul>
<li>Run <code>npm run dev</code> in <code>web/</code> for the Vite dev server (port 5173).</li>
<li>Or build the production bundle and rebuild with <code>make build</code>.</li>
</ul>
<p>API health check: <a href="/healthz"><code>/healthz</code></a></p>
</body>
</html>`

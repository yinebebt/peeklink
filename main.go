package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

//go:embed templates
var templateFS embed.FS

// safeHTML marks trusted HTML from server-fetched oEmbed responses for use in templates.
func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

var tmpl = template.Must(
	template.New("").Funcs(template.FuncMap{
		"safeHTML": safeHTML,
	}).ParseFS(templateFS, "templates/*.html"),
)

// Footer URL from -repo (trimmed, trailing slashes removed).
var footerRepoURL string

var limiters = map[Platform]*rate.Limiter{
	TikTok:    rate.NewLimiter(rate.Every(6*time.Second), 2),
	Instagram: rate.NewLimiter(rate.Every(6*time.Second), 2),
	YouTube:   rate.NewLimiter(rate.Every(3*time.Second), 3),
	Facebook:  rate.NewLimiter(rate.Every(6*time.Second), 2),
}

var videoCache = NewCache[*VideoResult](5 * time.Minute)

func main() {
	addr := flag.String("addr", ":8090", "HTTP listen address (host:port)")
	repo := flag.String("repo", "https://github.com/yinebebt/peeklink", "footer link for this project; empty hides")
	flag.Parse()

	footerRepoURL = strings.TrimSpace(strings.TrimSuffix(*repo, "/"))

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /view", handleView)
	mux.HandleFunc("POST /view", handleViewPost)

	slog.Info("listen", "addr", *addr)
	fmt.Fprintf(os.Stderr, "peeklink -> http://localhost%s\n", *addr)

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("listen stopped", "err", err)
		os.Exit(1)
	}
}

// footerData is embedded in page template data for templates/footer.html.
type footerData struct {
	RepoURL string
}

type indexData struct {
	footerData
}

type viewData struct {
	footerData
	Result *VideoResult
	Error  string
	URL    string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	setHTMLHeaders(w)

	data := indexData{
		footerData: footerData{RepoURL: footerRepoURL},
	}
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("template_error", "name", "index.html", "err", err)
	}
}

func handleViewPost(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	if rawURL == "" {
		slog.Info("view_post_empty")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	host := ""
	if u, err := url.Parse(rawURL); err == nil {
		host = u.Host
	}
	slog.Info("view_post", "host", host, "len", len(rawURL))
	http.Redirect(w, r, "/view?url="+url.QueryEscape(rawURL), http.StatusSeeOther)
}

func handleView(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		slog.Info("view_redirect_empty")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	data := viewData{
		footerData: footerData{RepoURL: footerRepoURL},
		URL:        rawURL,
	}

	if cached, ok := videoCache.Get(rawURL); ok {
		logResolvedView(rawURL, cached.Platform, cached.Strategy, true)
		data.Result = cached
		renderViewer(w, data)
		return
	}

	ctx := r.Context()
	result, err := Resolve(ctx, rawURL)
	if err != nil {
		slog.Error("resolve_failed", "err", err, "url", truncateLogString(rawURL))
		data.Error = "Could not load video. The link may be invalid or the platform is blocking access."
		renderViewer(w, data)
		return
	}

	ttl := time.Hour
	if result.VideoURL != "" && result.EmbedHTML == "" {
		ttl = 10 * time.Minute
	}
	videoCache.Set(rawURL, result, ttl)

	logResolvedView(rawURL, result.Platform, result.Strategy, false)
	data.Result = result
	renderViewer(w, data)
}

func logResolvedView(rawURL string, platform Platform, strategy string, fromCache bool) {
	host := ""
	if u, err := url.Parse(rawURL); err == nil {
		host = u.Host
	}
	slog.Info("view",
		"host", host,
		"platform", string(platform),
		"strategy", strategy,
		"cache_hit", fromCache,
	)
}

const truncateLogURLLen = 120

// truncateLogString limits string length in structured logs.
func truncateLogString(s string) string {
	if len(s) <= truncateLogURLLen {
		return s
	}
	return s[:truncateLogURLLen] + "..."
}

func renderViewer(w http.ResponseWriter, data viewData) {
	setHTMLHeaders(w)
	if err := tmpl.ExecuteTemplate(w, "viewer.html", data); err != nil {
		slog.Error("template_error", "name", "viewer.html", "err", err)
	}
}

func setHTMLHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

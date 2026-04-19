package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Platform identifies a supported video source.
type Platform string

const (
	TikTok    Platform = "tiktok"
	Instagram Platform = "instagram"
	YouTube   Platform = "youtube"
	Unknown   Platform = "unknown"
)

// VideoResult holds resolved video data ready for rendering.
type VideoResult struct {
	Platform     Platform
	ContentID    string
	CanonicalURL string
	VideoURL     string // direct MP4 - plays in native <video>
	EmbedHTML    string // platform embed snippet
	Thumbnail    string
	Title        string
	Author       string
	Strategy     string
	// YouTubeShort selects a vertical layout when the URL is a /shorts/ link.
	YouTubeShort bool
}

// DisplayName returns a human-friendly label for the platform.
func (p Platform) DisplayName() string {
	switch p {
	case TikTok:
		return "TikTok"
	case Instagram:
		return "Instagram"
	case YouTube:
		return "YouTube"
	default:
		return string(p)
	}
}

var (
	tiktokVideoRe = regexp.MustCompile(`tiktok\.com/@[\w.]+/video/(\d+)`)
	tiktokShortRe = regexp.MustCompile(`(?:vm|vt)\.tiktok\.com/[\w]+`)
	tiktokPhotoRe = regexp.MustCompile(`tiktok\.com/@[\w.]+/photo/(\d+)`)

	instaReelRe = regexp.MustCompile(`instagram\.com/(?:reel|reels)/([A-Za-z0-9_-]+)`)
	instaPostRe = regexp.MustCompile(`instagram\.com/p/([A-Za-z0-9_-]+)`)

	ytWatchRe  = regexp.MustCompile(`youtube\.com/watch\?.*v=([A-Za-z0-9_-]{11})`)
	ytShortRe  = regexp.MustCompile(`youtu\.be/([A-Za-z0-9_-]{11})`)
	ytShortsRe = regexp.MustCompile(`youtube\.com/shorts/([A-Za-z0-9_-]{11})`)
	ytEmbedRe  = regexp.MustCompile(`youtube\.com/embed/([A-Za-z0-9_-]{11})`)
	ytLiveRe   = regexp.MustCompile(`youtube\.com/live/([A-Za-z0-9_-]{11})`)

	ytPatterns = []*regexp.Regexp{ytWatchRe, ytShortRe, ytShortsRe, ytEmbedRe, ytLiveRe}
)

func detectPlatform(rawURL string) (Platform, string) {
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Host != "" {
		host := normalizeHost(parsed.Host)

		switch host {
		case "vm.tiktok.com", "vt.tiktok.com":
			return TikTok, ""
		case "tiktok.com":
			if id := matchPathSegments(parsed.EscapedPath(), "@", "video"); id != "" {
				return TikTok, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "@", "photo"); id != "" {
				return TikTok, id
			}
		case "instagram.com":
			if id := matchPathSegments(parsed.EscapedPath(), "reel"); id != "" {
				return Instagram, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "reels"); id != "" {
				return Instagram, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "p"); id != "" {
				return Instagram, id
			}
		case "youtube.com":
			if id := strings.TrimSpace(parsed.Query().Get("v")); len(id) == 11 {
				return YouTube, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "shorts"); id != "" {
				return YouTube, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "embed"); id != "" {
				return YouTube, id
			}
			if id := matchPathSegments(parsed.EscapedPath(), "live"); id != "" {
				return YouTube, id
			}
		case "youtu.be":
			segments := splitPathSegments(parsed.EscapedPath())
			if len(segments) > 0 && len(segments[0]) == 11 {
				return YouTube, segments[0]
			}
		}

		return Unknown, ""
	}

	// Fall back to the looser regex matcher for malformed or scheme-less input.
	if m := tiktokVideoRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return TikTok, m[1]
	}
	if tiktokShortRe.MatchString(rawURL) {
		return TikTok, ""
	}
	if m := tiktokPhotoRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return TikTok, m[1]
	}
	if m := instaReelRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return Instagram, m[1]
	}
	if m := instaPostRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return Instagram, m[1]
	}
	for _, re := range ytPatterns {
		if m := re.FindStringSubmatch(rawURL); len(m) > 1 {
			return YouTube, m[1]
		}
	}
	return Unknown, ""
}

func isShortLink(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Host != "" {
		switch normalizeHost(parsed.Host) {
		case "vm.tiktok.com", "vt.tiktok.com":
			return true
		}
	}
	return tiktokShortRe.MatchString(rawURL)
}

// allowedRedirectHosts prevents SSRF by only following redirects for known short-link domains.
var allowedRedirectHosts = map[string]bool{
	"vm.tiktok.com": true,
	"vt.tiktok.com": true,
}

// shortLinkCache stores resolved short-link to canonical-URL mappings.
// These rarely change, so a long TTL is fine.
var shortLinkCache = NewCache[string](30 * time.Minute)

// Resolve normalizes the input URL and returns the best available playback
// strategy for the detected platform.
func Resolve(ctx context.Context, rawURL string) (*VideoResult, error) {
	canonical := rawURL

	if isShortLink(rawURL) {
		if cached, ok := shortLinkCache.Get(rawURL); ok {
			canonical = cached
			slog.Info("short link cache hit", "from", rawURL, "to", canonical)
		} else {
			resolved, err := followRedirects(ctx, rawURL)
			if err != nil {
				slog.Warn("short link resolution failed, using original", "url", rawURL, "err", err)
			} else {
				canonical = resolved
				shortLinkCache.Set(rawURL, canonical, 24*time.Hour)
				slog.Info("resolved short link", "from", rawURL, "to", canonical)
			}
		}
	}

	plat, contentID := detectPlatform(canonical)
	slog.Info("detected", "platform", plat, "id", contentID, "url", canonical)

	if lim, ok := limiters[plat]; ok {
		if err := lim.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limited: %w", err)
		}
	}

	switch plat {
	case TikTok:
		return fetchTikTok(ctx, canonical, contentID)
	case Instagram:
		return fetchInstagram(canonical, contentID)
	case YouTube:
		return fetchYouTube(ctx, canonical, contentID)
	default:
		return &VideoResult{Platform: Unknown, CanonicalURL: canonical, Strategy: "direct-link"}, nil
	}
}

const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

func followRedirects(ctx context.Context, rawURL string) (string, error) {
	// SSRF protection: only follow redirects for known short-link domains.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	host := normalizeHost(parsed.Host)
	if !allowedRedirectHosts[host] {
		return "", fmt.Errorf("redirect not allowed for host %q", parsed.Hostname())
	}

	// Apply a generous timeout because TikTok short links can be slow to resolve.
	// Derive from parent ctx so HTTP-client cancellation still propagates.
	resolveCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects (%d)", len(via))
			}
			return nil
		},
	}
	// Use GET because some platforms do not respond to HEAD on redirect endpoints.
	req, err := http.NewRequestWithContext(resolveCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// Drain the body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Request.URL.String(), nil
}

func fetchJSON(ctx context.Context, endpoint string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func normalizeHost(host string) string {
	host = strings.ToLower(host)
	if trimmedHost, _, err := net.SplitHostPort(host); err == nil {
		host = trimmedHost
	}
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	return host
}

func splitPathSegments(path string) []string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		filtered = append(filtered, segment)
	}
	return filtered
}

func matchPathSegments(path string, prefix string, kind ...string) string {
	segments := splitPathSegments(path)
	if len(segments) == 0 {
		return ""
	}

	switch {
	case prefix == "@" && len(segments) >= 3 && strings.HasPrefix(segments[0], "@"):
		for _, value := range kind {
			if segments[1] == value {
				return segments[2]
			}
		}
	case prefix != "" && len(segments) >= 2 && segments[0] == prefix:
		return segments[1]
	}

	return ""
}

// fetchTikTok resolves a TikTok URL using oEmbed metadata when available.
// The older og:video tag is no longer reliably present in SSR HTML.

func fetchTikTok(ctx context.Context, canonical, contentID string) (*VideoResult, error) {
	var oembed struct {
		Title  string `json:"title"`
		Author string `json:"author_name"`
		HTML   string `json:"html"`
		Thumb  string `json:"thumbnail_url"`
	}
	endpoint := "https://www.tiktok.com/oembed?url=" + url.QueryEscape(canonical)
	if err := fetchJSON(ctx, endpoint, &oembed); err == nil && oembed.HTML != "" {
		// Render oEmbed HTML directly in the page. TikTok's embed.js will
		// replace the blockquote with its own sandboxed iframe video player.
		// This is the standard way every website embeds TikTok videos.
		return &VideoResult{
			Platform: TikTok, ContentID: contentID, CanonicalURL: canonical,
			EmbedHTML: oembed.HTML, Thumbnail: oembed.Thumb,
			Title: oembed.Title, Author: oembed.Author,
			Strategy: "tiktok-oembed",
		}, nil
	}
	slog.Warn("tiktok oembed failed, falling back to direct link", "url", canonical)

	return &VideoResult{
		Platform: TikTok, ContentID: contentID, CanonicalURL: canonical,
		Strategy: "tiktok-direct-link",
	}, nil
}

// fetchInstagram builds the safest available public embed for Instagram content.
// The embed iframe is the only reliable server-side option, and the sandbox
// prevents Instagram's JS from navigating the user away.

func fetchInstagram(canonical, contentID string) (*VideoResult, error) {
	if contentID == "" {
		return &VideoResult{
			Platform: Instagram, CanonicalURL: canonical,
			Strategy: "instagram-direct-link",
		}, nil
	}

	// Instagram's /embed/ endpoint renders a working video player for public reels.
	// The sandbox attribute prevents the iframe JS from redirecting the user to
	// instagram.com when they click, so the video keeps playing inline instead.
	embedURL := fmt.Sprintf("https://www.instagram.com/reel/%s/embed/captioned/", contentID)
	embedHTML := fmt.Sprintf(
		`<iframe src="%s" width="400" height="780" `+
			`title="Instagram video" loading="lazy" scrolling="no" `+
			`referrerpolicy="strict-origin-when-cross-origin" `+
			`sandbox="allow-scripts allow-same-origin allow-popups" `+
			`style="max-width:100%%; border:0; border-radius:12px;"></iframe>`,
		embedURL,
	)
	return &VideoResult{
		Platform: Instagram, ContentID: contentID, CanonicalURL: canonical,
		EmbedHTML: embedHTML,
		Strategy:  "instagram-embed",
	}, nil
}

// fetchYouTube returns metadata plus a lazy youtube.com embed strategy
// (thumbnail first; iframe on click in templates/viewer.html). Plain embed URLs
// tend to behave more reliably than privacy-enhanced nocookie with extra params.
func fetchYouTube(ctx context.Context, canonical, contentID string) (*VideoResult, error) {
	if contentID == "" {
		return &VideoResult{
			Platform: YouTube, CanonicalURL: canonical,
			Strategy: "youtube-direct-link",
		}, nil
	}

	var oembed struct {
		Title  string `json:"title"`
		Author string `json:"author_name"`
		Thumb  string `json:"thumbnail_url"`
	}
	endpoint := "https://www.youtube.com/oembed?url=" + url.QueryEscape(canonical) + "&format=json"
	_ = fetchJSON(ctx, endpoint, &oembed) // best-effort metadata

	thumb := oembed.Thumb
	if thumb == "" {
		thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", url.PathEscape(contentID))
	}

	return &VideoResult{
		Platform: YouTube, ContentID: contentID, CanonicalURL: canonical,
		Thumbnail: thumb,
		Title:     oembed.Title, Author: oembed.Author,
		Strategy:     "youtube-lazy-embed",
		YouTubeShort: strings.Contains(strings.ToLower(canonical), "/shorts/"),
	}, nil
}

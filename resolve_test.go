package main

import "testing"

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantPlat Platform
		wantID   string
	}{
		{"tiktok video", "https://www.tiktok.com/@user/video/7234567890123456789", TikTok, "7234567890123456789"},
		{"tiktok short vm", "https://vm.tiktok.com/ZMrKxyz123/", TikTok, ""},
		{"tiktok short vt", "https://vt.tiktok.com/ZSrAbcDef/", TikTok, ""},
		{"instagram reel", "https://www.instagram.com/reel/CxABCdef123/", Instagram, "CxABCdef123"},
		{"instagram mobile reel", "https://m.instagram.com/reel/CxABCdef123/", Instagram, "CxABCdef123"},
		{"instagram reels", "https://www.instagram.com/reels/CxABCdef123/", Instagram, "CxABCdef123"},
		{"instagram post", "https://www.instagram.com/p/CxABCdef123/", Instagram, "CxABCdef123"},
		{"instagram reel with params", "https://www.instagram.com/reel/DXExjj0immR/?igsh=MWx3ejkzdmo2ZHFzeg==", Instagram, "DXExjj0immR"},
		{"youtube watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube mobile watch", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube watch without scheme", "youtube.com/watch?v=dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube short", "https://youtu.be/dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube live", "https://www.youtube.com/live/dQw4w9WgXcQ", YouTube, "dQw4w9WgXcQ"},
		{"youtube watch+params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s", YouTube, "dQw4w9WgXcQ"},
		{"tiktok photo", "https://www.tiktok.com/@user/photo/7234567890123456789", TikTok, "7234567890123456789"},
		{"tiktok video dotted user", "https://www.tiktok.com/@user.name/video/1234567890123456789", TikTok, "1234567890123456789"},
		{"youtube watch v first param", "https://www.youtube.com/watch?v=abcDEF12345&list=PLxyz", YouTube, "abcDEF12345"},
		{"query string should not impersonate platform", "https://example.com/?next=https://www.youtube.com/watch?v=dQw4w9WgXcQ", Unknown, ""},
		{"unknown", "https://example.com/video", Unknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat, id := detectPlatform(tt.url)
			if plat != tt.wantPlat {
				t.Errorf("platform = %q, want %q", plat, tt.wantPlat)
			}
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestIsShortLink(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://vm.tiktok.com/ZMrKxyz123/", true},
		{"https://vt.tiktok.com/ZSrAbcDef/", true},
		{"https://www.tiktok.com/@user/video/123", false},
		{"https://www.instagram.com/reel/abc/", false},
	}
	for _, tt := range tests {
		if got := isShortLink(tt.url); got != tt.want {
			t.Errorf("isShortLink(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

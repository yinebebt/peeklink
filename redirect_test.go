package main

import (
	"net/url"
	"testing"
)

func TestRedirectHopAllowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want bool
	}{
		{"https://www.tiktok.com/@x/video/1", true},
		{"https://vm.tiktok.com/abc", true},
		{"https://www.youtube.com/watch?v=1", true},
		{"https://youtu.be/1", true},
		{"https://127.0.0.1/", false},
		{"http://169.254.169.254/latest/meta-data/", false},
		{"https://evil.example/", false},
		{"ftp://tiktok.com/", false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			u, err := url.Parse(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			got := redirectHopAllowed(u) == nil
			if got != tc.want {
				t.Fatalf("redirectHopAllowed(%q) allowed = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

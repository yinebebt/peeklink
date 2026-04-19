# peeklink

peeklink is a small self-hosted web app for opening public video links without the social-media detour.

I built it because I do not want to keep social accounts around just to watch a link someone sent me. When a friend shares a TikTok, Instagram, or YouTube video, I want a clean browser view with no account prompts, no app-install nudges, and no feed noise.

## What It Does

Paste a public TikTok, Instagram, or YouTube link and peeklink tries to show the video in a clean viewer. 
If a platform blocks playback, the app falls back to the canonical link. 

Supports TikTok, Instagram Reels, and YouTube links with short-link resolution, in-memory caching, per-platform rate limiting, and a single-binary, no-database setup.

## How it Works

The resolver follows a simple fallback chain:

1. Normalize the URL
2. Detect the platform
3. Check the cache
4. Try the best available playback strategy
5. Fall back to a canonical link if the platform blocks playback

## Getting Started

### Run locally

```bash
go run . --addr :8090
```

Then open:

```text
http://localhost:8090
```

Footer links live in `templates/footer.html`; override with `--repo` and `--deeplink`, or pass empty strings to hide them.

## Limits & Roadmap

- Private posts are not supported
- Does not bypass platform authentication
- Not intended for downloading or rehosting media

- Improve playback reliability and provide clearer errors when blocked
- Expand supported sources while keeping the “peek, don’t replace the app” scope

## License

[MIT](./LICENSE)

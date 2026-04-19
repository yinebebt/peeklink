# peeklink

peeklink is a small self-hosted web app for opening public video links without the social-media detour.

I built it because I do not want to keep social accounts around just to watch a link someone sent me. When a friend shares a TikTok, Instagram, or YouTube video, I want a clean browser view with no account prompts, no app-install nudges, and no feed noise.

## What It Does

Paste a public TikTok, Instagram, or YouTube link and peeklink tries to show the video in a clean viewer.

If a platform blocks playback, the app falls back to the canonical link.

## Demo

1. Open the homepage.
2. Paste a public TikTok, Instagram, or YouTube link.
3. Watch the clean viewer page load.

## Features

- Supports public TikTok, Instagram Reel, and YouTube links
- Resolves short TikTok links before rendering
- Uses in-memory caching to reduce repeat requests
- Applies simple rate limiting per platform
- Runs as a single Go binary
- No database or external backend service

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
go run .
```

Then open:

```text
http://localhost:8090
```

### Change the port

```bash
go run . --addr :8080
```

Footer links live in `templates/footer.html`; override with `--repo` and `--deeplink`, or pass empty strings to hide them.

### Build a binary

```bash
go build -o peeklink .
```

### Run tests

```bash
go test ./...
```

## Supported Sources

- TikTok
- Instagram Reels and posts
- YouTube watch, short, shorts, embed, and live URLs

## Known Limits

- Private posts are not supported.
- Some YouTube links can still trigger platform-side bot checks depending on origin or browser.
- This tool does not bypass platform authentication.
- It is designed for viewing public links in the browser, not downloading or rehosting media.

## Roadmap

- Improve playback reliability and clearer errors when a platform blocks the viewer
- Broader source support where it still fits the “peek, don’t replace the app” idea

## Contributing

Pull requests are welcome. Please keep changes small, dependency-light, and covered by tests when behavior changes.

## License

[MIT](./LICENSE)

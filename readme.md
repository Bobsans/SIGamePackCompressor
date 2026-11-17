# SIGame Pack Compressor

Go backend and Vue frontend for compressing SIGame packages.

- Analyze SIQ versions 3, 3.1, 4 and 5 before processing
- Convert to v5 or preserve the source version
- Resize/re-encode images with libvips, including GIF/WebP animation
- Configure audio/video compression through FFmpeg
- Name media by MD5, deduplicate resources and remove unused attachments
- Follow individual WebSocket updates in a complete scrollable journal
- Cancel jobs, reuse uploads and download results

## Development

Install Go, Node.js, FFmpeg and libvips. Copy `config.example.yaml` to
`config.yaml` in the repository root. Then:

```sh
cd api
go run ./cmd/server -config ../config.yaml
```

In another terminal:

```sh
cd app
npm ci
npm run dev
```

The backend listens on `127.0.0.1:8000`. The example config permits the local
Vite frontend. Storage lives in the root `storage/` directory.

## Build

Build the server from `api` with
`go build -o ../sigame-compressor ./cmd/server`.
Build the frontend from `app` with `npm run build`, setting
`API_SERVER_URL=/` for same-origin hosting. Serve the frontend build from
`web/` beside the executable, or set `web_path` explicitly.

The executable defaults to the adjacent `config.yaml`, `storage/` and
`web/`. Service/proxy configuration belongs to the host environment.

See [backend and API](docs/backend.md), [frontend workflow](docs/frontend.md),
and [default field dimensions](docs/game-field-size.md).

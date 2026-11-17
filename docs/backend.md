# Backend

The backend is a Go HTTP server. Images use the libvips tools (`vips`,
`vipsheader`); audio/video use `ffmpeg` and `ffprobe`. Python and CGo are not
required. The libvips installation must support JPEG, PNG, GIF, WebP and AVIF.

## Run and storage

Build from `api` with `go build -o ../sigame-compressor ./cmd/server`.
The executable looks for `config.yaml` beside itself. Copy the root
`config.example.yaml` to `config.yaml` and adjust it as needed.

Relative storage and web paths resolve against the config file's directory,
independently of the working directory. The default layout is:

```text
sigame-compressor
config.yaml
storage/
web/
```

The default listener is `127.0.0.1:8000`. Environment overrides:
`SI_LISTEN`, `SI_STORAGE_PATH`, `SI_WEB_PATH`, `SI_VIPS`,
`SI_VIPSHEADER`, `SI_FFMPEG`, `SI_FFPROBE`.
Configure exact `allowed_origins` for a separate development frontend;
same-origin requests need no extra setting.

For development without a persistent executable, run from `api`:
`go run ./cmd/server -config ../config.yaml`.
The explicit config path prevents Go's temporary executable directory from
becoming the storage location.

`GET /health` returns the build version and tool availability. `--version`
prints the build version without starting the server. Availability is not a
codec test; the native integration tests exercise actual formats.

## API workflow

Upload/job IDs are random capability identifiers: possession grants access.
There is no account-based authentication. Do not serve storage as static files.
Errors contain `error` and `detail`.

1. `GET /api/options`: defaults, supported versions/formats and resource limits
2. `POST /api/uploads`: multipart form containing one `file` with a `.siq`
   extension; upload and analyze only
3. `GET /api/uploads/{id}`: saved title, source version, counts, media details,
   warnings, and unused-file count/size
4. `POST /api/uploads/{id}/jobs`: create a job with optional settings JSON;
   omitted fields inherit defaults, including nested fields
5. `GET /api/jobs/{id}`: current state, full file log, progress and result URL
6. `GET /api/jobs/{id}/ws`: live WebSocket updates
7. `POST /api/jobs/{id}/cancel`: cancel queued/running work
8. `GET /api/jobs/{id}/download`: download a completed result

An upload can create independent jobs with different settings. Original uploads
are immutable. The old Python `/compress`, `/ws` and `/download/{hash}`
interfaces are not exposed.

### WebSocket

The first message is `{type:"snapshot", state:...}` with full history. Subsequent
messages are `{type:"file", entry:...}` for individual file starts/completions
and `{type:"status", state:...}` without history. A file's `sequence`
identifies its row. Subscription and initial snapshot capture are atomic.

Slow readers are disconnected with a retry code and recover from a fresh
snapshot. The connection closes normally after terminal status. Ping/pong
keeps long encodes connected. The frontend does not poll HTTP for progress.
Any reverse proxy must forward WebSocket upgrades for `/api/jobs/{id}/ws`.

Log entries contain `sequence`, `name`, `type`, `status`, `old_size`,
`started_at`, and optional `new_size` and `warning`. File status is
`processing`, `compressed`, `unchanged` or `interrupted`.

## Compression options

```json
{
  "output_version": "5",
  "images": {
    "enabled": true,
    "max_width": 1152,
    "max_height": 658,
    "quality": 80,
    "format": "webp",
    "lossless": false,
    "animation": "preserve"
  },
  "audio": {"enabled": true, "format": "mp3", "bitrate_kbps": 96},
  "video": {
    "enabled": true,
    "max_width": 1152,
    "max_height": 658,
    "crf": 26,
    "preset": "medium",
    "audio_bitrate_kbps": 96
  }
}
```

Output package version is `5` or `source`; downgrading v5 is not supported.
Image format is `webp` or `source`. Animation mode `preserve` keeps the
original animation format; `webp` permits animated WebP. GIF/WebP frames and
timing are preserved. Other animated formats are retained with a warning.
Audio output is MP3 or Opus. Video output is MP4/H.264/yuv420p with AAC.

Images and videos fit proportionally within editable bounds without cropping
or upscaling. The default 1152 x 658 box comes from the normal SIGame layout
at a 1920 x 1080 viewport; see [the measurement](game-field-size.md).
Video scaling also accounts for non-square source pixels.

An encoded resource that is larger than its source is discarded in favor of
the original. A requested output format is therefore not a forced conversion.
Whole-archive size can still grow when package structure changes.

## Package processing

Source versions 3, 3.1, 4 and 5 are accepted. Conversion to v5 follows the
upstream SIPackages rules at commit
`eaf15e4cbaa414c827c48bc751532ba58e2cac73`: question types, price ranges,
answer markers, placement, timing, references and legacy author/source records.
Unknown legacy scenario elements and mixed legacy/modern questions fail explicitly.

Only referenced assets and required package metadata remain in results.
Question, answer, script, logo and static HTML/CSS/SVG/relationship references
are followed. Unused media and stray files are removed.

Processed media use the MD5 of their final bytes plus extension, including
original bytes retained after a failed/larger conversion. Identical resources
are stored once; a separate SHA-256 comparison rejects differing contents with
the same MD5 filename. MD5 is used for naming, not security. XML/static asset
references and content types follow renames; stale optional v5 hashes are removed.

Executable HTML, dynamic imports, base URLs and compound srcsets cannot be
resolved statically. Reachable content with those constructs retains archive
assets and names conservatively, with an explicit warning. Unreferenced HTML
does not trigger this exception.

## Persistence

`storage/db.sipc` contains `go_uploads`, `go_jobs` and `go_job_files`.
Each file log row is upserted separately, so every event does not rewrite the
entire history. Originals, results and intermediates are under `uploads/`,
`results/` and `tmp/`. Results are published only after a complete ZIP write.

A server restart marks queued/running jobs failed and active log rows interrupted;
their uploads can be reused. Completed jobs remain downloadable. Use one server
instance per storage directory. Retention is manual; storage is never deleted by
the application or release workflow.

## Validation

From `api`:

```sh
go test ./...
go vet ./...
SI_MEDIA_TESTS=1 go test ./internal/media -count=1 -v
```

Native tests cover formats, alpha, GIF/WebP timing, proportional resize,
non-square video pixels, MP3 and H.264. Other tests cover SIQ conversion,
cleanup, deduplication, MD5 collision handling, complete journal persistence,
WebSocket events/reconnection, cancellation and origin checks.

The v3/3.1 tests use constructed legacy fixtures. Real v4/v5 packages were also
checked with the official SIPackages reader: all 164 questions and 150 references
in the v4 sample survived conversion with equivalent question semantics.

## Release

GitHub Actions tests native codecs, runs Go race tests and builds the frontend
and Linux binary. The build embeds its commit SHA in `/health`.
Repository variables: `API_SERVER_URL` (`/` for same origin),
`GO_BACKEND_READY`, `GO_SERVICE_NAME`, `SSH_PATH`.
The production environment supplies SSH credentials.

The service and reverse-proxy configuration are maintained on the server,
not shipped with the application. Releases replace the executable and `web/`
without touching the adjacent `storage/` or `config.yaml`.

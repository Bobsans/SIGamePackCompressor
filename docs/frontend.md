# Frontend workflow

The Vue interface uses the Go upload/job API. Its four stages are upload,
settings, processing and result. Styling retains the navy/lavender palette,
with gold accents and board-like numeric summaries inspired by SIGame.

## Local development

Run the Go server with the development origins in the root `config.example.yaml`,
then run `npm ci` and `npm run dev` from `app`. The API URL defaults to
`http://localhost:8000`. Set `API_SERVER_URL` at build time for another endpoint;
an empty string uses same-origin paths. For a same-origin preview, build the
frontend and point the Go server's `web_path` at the absolute `app/dist` path.

## Behavior

- Files are checked for `.siq`, nonzero size, and the server's upload limit
- Upload progress switches to an indeterminate analysis state after transfer
- Package title/version, rounds, themes, questions, media sizes and metadata are shown
- Each media type has an independent switch and settings; absent media and
  unavailable processing tools disable the corresponding controls
- Output format is version 5 or the source version; text-only legacy packages
  can be converted with media processing disabled
- Job updates arrive over WebSocket: one file event per start/completion and
  separate status events. A reconnect obtains a complete snapshot; no periodic
  HTTP status requests are made
- The complete file log stays in a fixed-height scroll area, newest records
  first. A bottom gradient fades rows leaving the viewport, rather than hiding
  history. Reading old rows preserves scroll position as new entries arrive
- Cancelling a job preserves the upload; changing settings creates a new job
- Session storage restores the current upload, edited settings and job after
  reloading the same tab; it is not an account history or cross-tab library
- Terminal states provide a download link or retry actions; larger results
  are reported without claiming a size saving
- Analysis shows the number/size of unused attachments that can be removed;
  completed results report the actual cleanup count

Main files: `App.vue` composes the stages; `useCompression.ts` owns request and
session lifecycle; `CompressionSettings.vue`, `PackSummary.vue`, and
`JobPanel.vue` render their respective parts. `api.ts` contains the typed API.

## Verification, 2026-11-17

`npm run build` passed (strict Vue/TypeScript checks and Vite production build).
The built app was exercised against an isolated local Go-server storage:

- Real SIQ upload, analysis, changed dimensions/bitrate/preset, compression and result
- Edited settings and running-job restoration after tab reload
- Cancellation and returning to settings without another upload
- Disabled image processing with an unfinished numeric edit
- Actual server interruption, connection warning, automatic status recovery,
  interrupted-job error and retry action
- Wrong file extension rejected before upload
- Text-only version 3 package: disabled media controls, no-op action disabled
  for source format, conversion to v5, and downloaded ZIP/XML verified
- Mobile layout at 390 px, with no horizontal page overflow, and desktop layout

No production deployment was performed. Native game playback is covered by
the separate backend verification scope, not this UI check.

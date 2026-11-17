# Default media dimensions

Images and videos default to a shared **1152 x 658** bounding box. Each file is
reduced proportionally to fit within both limits, with no crop or padding and
no enlargement of smaller inputs. H.264 output dimensions are rounded to even
pixels. Both limits remain editable in the UI (16..8192 px).

## Previous Python behavior

At compressor commit `64cd345d32801c58b4ee476857a5aa94da3f2a1f`,
`api/compress/ops/__init__.py` limited images to **800 x 800** via
`min(800 / width, 800 / height)` and videos to **1024 x 664** using FFmpeg's
aspect-preserving scale filter. These were compressor settings, not SIQ limits.

## Live measurement, 2026-11-17

Measured the official client at https://sigame.vladimirkhil.com/ using its local
"How to play" demo, with three players, ordinary layout (not deep mode), a
1920 x 1080 content viewport, devicePixelRatio 1 and 100% scale. This models
the full-screen application area, excluding browser chrome and OS taskbars.
`getBoundingClientRect()` returned the following CSS-pixel dimensions:

| Element | Width | Height |
| --- | ---: | ---: |
| Whole game `.gameMain` | 1920 | 1080 |
| Left game area `.game__tableArea` | 1551.515625 | 1080 |
| Players `.playersPanel` | 1551.515625 | 280 |
| Host panel `.showmanProgressArea` | 370 | 800 |
| Board and controls `.tableArea` | 1181.515625 | 800 |
| Board `#table` | 1181.515625 | 732 |
| Caption `.tableCaption` | 1181.515625 | 45 |
| Content including border `.tableContent` | 1181.515625 | 687 |
| Actual content `.table-content` / `.mainContent` | 1153.515625 | 659 |

The transparent board border occupies 14 px on each side. Rounding the inner
content box down to even integers gives **1152 x 658** for both image and video
defaults. A 1920 x 1080 landscape input becomes 1152 x 648, not 1152 x 658;
a square input becomes 658 x 658; a 640 x 360 input stays 640 x 360.

Relevant upstream sources at SIOnline commit
`a3da37b4744d323acebe5a5fc22e4e70e783b635`:

- [Room layout](https://github.com/VladimirKhil/SIOnline/blob/a3da37b4744d323acebe5a5fc22e4e70e783b635/src/components/views/Room/Room.css)
- [Players sizing](https://github.com/VladimirKhil/SIOnline/blob/a3da37b4744d323acebe5a5fc22e4e70e783b635/src/components/game/PlayersView/PlayersView.scss)
- [Caption and board](https://github.com/VladimirKhil/SIOnline/blob/a3da37b4744d323acebe5a5fc22e4e70e783b635/src/components/gameTable/GameTable/GameTable.css)
- [Content border](https://github.com/VladimirKhil/SIOnline/blob/a3da37b4744d323acebe5a5fc22e4e70e783b635/src/components/gameTable/TableBorder/TableBorder.css)

These are a practical default for this layout, not a universal SIGame maximum.
Window chrome, zoom/DPI, client version, host controls, deep mode and split
question layouts affect the available area. Custom saved settings are not
overwritten; use "Reset settings" or upload a new package to apply new defaults.

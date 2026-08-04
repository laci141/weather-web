# 🌦️ Weather-Web V3

Historical weather viewer + data downloader for Open-Meteo (daily/hourly/3-hour/15-min resolution).

## Features

- **Current Weather**: Live conditions plus a 7-day outlook, exportable
- **Historical View**: 1940–today, max 5 weeks display, weekly charts + tables, exportable on the spot (including all charts as one PNG)
- **Data Downloader**: Any date range (1940–today), multiple resolutions, XLSX/CSV/JSON/Markdown export
- **Climate Anomaly**: Any year's monthly means against the 1991–2020 WMO normal, with a warm/cool chart
- **Date entry**: Explicit Year / Month / Day controls — no OS calendar on mobile, and the month can never be read as the day
- **Quick ranges**: Last 30 days, this year, last year, 1940 → today
- **Units**: European primary (°C, km/h, mm) with toggle to Fahrenheit/mph/inches
- **Remembers**: City, units, resolution, field selection and dates persist across reloads
- **Mobile-friendly**: Glass-panel UI, responsive design (tested on Huawei Mate 10 Pro)

### Markdown export

`.md` is a readable digest rather than a copy of the rows: a metadata header
(location, range, resolution, units, source) followed by **yearly, monthly and
ISO-weekly** aggregate tables. Each column is aggregated by its own rule —
`sum` for precipitation and sunshine, `max`/`min` for daily extremes, a
**circular mean** for wind direction (350° and 10° average to 0°, not 180°),
and the arithmetic mean for everything else. A `Points` column shows how many
readings each period holds, so partial weeks and months at the edges of a range
are visible instead of silently averaged.

## Live

https://weather-web-xxxxx.onrender.com/ (Render free tier)

## Tech Stack

- **Backend**: Go 1.26, Open-Meteo Archive + Forecast API (free, keyless)
- **CLI**: weather-goat from [mvanhorn/printing-press-library](https://github.com/mvanhorn/printing-press-library)
- **Frontend**: Vanilla JS, Chart.js, SheetJS (XLSX/CSV)
- **Deploy**: Cloudflare Pages (static + Functions), or Docker multi-stage (Alpine) → Render

## Layout

```
public/index.html          static site (the whole UI)
functions/api/[[path]].js  /api/* on Cloudflare Pages (JS, runs at the edge)
main.go                    /api/* for local dev + Docker/Render (Go)
```

Both backends expose the same routes and the same JSON shapes, so
`public/index.html` is identical in either deployment.

## Deploy to Cloudflare Pages

Workers & Pages → Create → Pages → Connect to Git → pick this repo, then:

| Setting | Value |
| --- | --- |
| Production branch | `main` |
| Framework preset | `None` |
| Build command | *(leave empty)* |
| Build output directory | `public` |
| Root directory | *(leave empty)* |
| Environment variables | *(none)* |

No build step and no API key are needed — Pages uploads `public/` as static
assets and compiles `functions/` automatically. Open-Meteo is keyless.

## Building Locally

```bash
cd weather-web
go build -o bin/server.exe .
CLI_BIN=./bin/weather-goat-pp-cli.exe PORT=8097 ./bin/server.exe
# Open http://localhost:8097
```

To run the Cloudflare version locally instead (static assets + Functions, no Go):

```bash
npx wrangler pages dev public
# Open http://localhost:8788
```

## Credits & License

- **Weather-Web** (this repo): MIT
- **Open-Meteo API**: Free, no auth required
- **weather-goat CLI**: [mvanhorn/printing-press-library](https://github.com/mvanhorn/printing-press-library) (upstream open-source)

See [mvanhorn/printing-press-library/LICENSE](https://github.com/mvanhorn/printing-press-library) for upstream license.

## Author

Laci (laci141) — July 2026

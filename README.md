# 🌦️ Weather-Web V3

Historical weather viewer + data downloader for Open-Meteo (daily/hourly/3-hour/15-min resolution).

## Features

- **Current Weather**: Real-time forecast (Oradea, Budapest, any city worldwide)
- **Historical View**: 1940–today, max 5 weeks display, split into weekly charts + tables
- **Data Downloader**: Any date range (1940–today), multiple resolutions, XLSX/CSV/JSON export
- **Units**: European primary (°C, km/h, mm) with toggle to Fahrenheit/mph/inches
- **Mobile-friendly**: Glass-panel UI, responsive design (tested on Huawei Mate 10 Pro)

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

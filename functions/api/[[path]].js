// Cloudflare Pages Function — serves every /api/* route.
//
// Cloudflare Pages runs static assets plus JavaScript Workers; it cannot run
// the Go server in main.go. This file is the Go backend's counterpart on the
// edge: same routes, same request bodies, same response shapes, so
// public/index.html works unchanged whether it is served by ./server locally
// or by Pages in production.
//
// The two endpoints that shell out to the weather-goat CLI in main.go
// (/api/forecast, /api/geocoding) call the Open-Meteo APIs the CLI wraps, and
// nest the payload under "results" the way the CLI's --json output does.

const ARCHIVE_BASE = 'https://archive-api.open-meteo.com/v1/archive';
const FORECAST_BASE = 'https://api.open-meteo.com/v1/forecast';
const GEOCODING_BASE = 'https://geocoding-api.open-meteo.com/v1/search';

const CORS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'POST, GET, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

const json = (data, status = 200) =>
  new Response(JSON.stringify(data), {
    status,
    headers: { ...CORS, 'Content-Type': 'application/json' },
  });

// Errors go out as plain text — index.html shows the body verbatim.
const fail = (message, status) =>
  new Response(message, {
    status,
    headers: { ...CORS, 'Content-Type': 'text/plain; charset=utf-8' },
  });

// --- units ---
// The unit spec arrives from the client. Every value is validated against an
// allow-list; anything else falls back to the European/metric default.

const tempUnit = (u) => (u && u.temperature === 'fahrenheit' ? 'fahrenheit' : 'celsius');
const windUnit = (u) => (u && u.wind === 'mph' ? 'mph' : 'kmh');
const precipUnit = (u) => (u && u.precipitation === 'inch' ? 'inch' : 'mm');

// --- allow-lists ---
// Only these field names may reach the upstream query string.

const DAILY_FIELDS = new Set([
  'temperature_2m_max', 'temperature_2m_min', 'temperature_2m_mean',
  'apparent_temperature_max', 'apparent_temperature_min',
  'relative_humidity_2m_max', 'relative_humidity_2m_min',
  'dew_point_2m_max', 'dew_point_2m_min',
  'precipitation_sum', 'rain_sum', 'snowfall_sum',
  'surface_pressure_mean', 'cloud_cover_mean',
  'wind_speed_10m_max', 'wind_gusts_10m_max', 'wind_direction_10m_dominant',
  'sunshine_duration', 'shortwave_radiation_sum',
]);

const HOURLY_FIELDS = new Set([
  'temperature_2m', 'apparent_temperature',
  'relative_humidity_2m', 'dew_point_2m',
  'precipitation', 'rain', 'snowfall',
  'surface_pressure', 'cloud_cover',
  'wind_speed_10m', 'wind_gusts_10m', 'wind_direction_10m',
  'sunshine_duration', 'shortwave_radiation',
]);

// minutely_15 supports a subset of the hourly variables (no surface_pressure,
// no cloud_cover).
const MINUTELY_FIELDS = new Set([
  'temperature_2m', 'apparent_temperature',
  'relative_humidity_2m', 'dew_point_2m',
  'precipitation', 'rain', 'snowfall',
  'wind_speed_10m', 'wind_gusts_10m', 'wind_direction_10m',
  'sunshine_duration', 'shortwave_radiation',
]);

const DAILY_DEFAULT = 'temperature_2m_max,temperature_2m_min,precipitation_sum,wind_speed_10m_max';
const HOURLY_DEFAULT = 'temperature_2m,relative_humidity_2m,precipitation,wind_speed_10m';

// dateRe strictly validates YYYY-MM-DD before the value goes into an outbound
// URL (defense against injection through the date fields).
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

// filterFields keeps only allow-listed names; returns def when nothing survives.
function filterFields(requested, allowed, def) {
  const keep = (Array.isArray(requested) ? requested : [])
    .map((f) => String(f).trim())
    .filter((f) => allowed.has(f));
  return keep.length ? keep.join(',') : def;
}

// Fetch upstream JSON, mapping every upstream failure onto a 502 the way
// main.go does. Open-Meteo reports parameter errors as HTTP 200 with
// {"error":true,...}, so those are surfaced as 502 too — otherwise the client
// would treat the payload as data.
async function fetchMeteo(url) {
  let resp;
  try {
    resp = await fetch(url, { headers: { Accept: 'application/json' } });
  } catch {
    return { error: fail('weather API unreachable', 502) };
  }

  const body = await resp.text();
  if (resp.status >= 400) return { error: fail(body, 502) };

  let data;
  try {
    data = JSON.parse(body);
  } catch {
    return { error: fail('invalid response from weather API', 502) };
  }
  if (data && data.error) return { error: fail(String(data.reason || 'weather API error'), 502) };

  return { data };
}

// --- history endpoints ---
// Validates the request, builds the upstream URL with the given resolution
// parameter (daily / hourly / minutely_15) and unit parameters, and returns the
// Open-Meteo payload unchanged.
async function proxyMeteo(req, base, param, allowed, def) {
  const lat = Number(req.latitude);
  const lon = Number(req.longitude);
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
    return fail('latitude and longitude must be numbers', 400);
  }
  if (!DATE_RE.test(String(req.start_date)) || !DATE_RE.test(String(req.end_date))) {
    return fail('start_date and end_date must be YYYY-MM-DD', 400);
  }

  const params = new URLSearchParams({
    latitude: lat.toFixed(6),
    longitude: lon.toFixed(6),
    start_date: req.start_date,
    end_date: req.end_date,
    [param]: filterFields(req.fields, allowed, def),
    timezone: 'auto',
    // unit params — European/metric defaults unless the client toggled
    temperature_unit: tempUnit(req.units),
    wind_speed_unit: windUnit(req.units),
    precipitation_unit: precipUnit(req.units),
  });

  const { data, error } = await fetchMeteo(`${base}?${params}`);
  return error || json(data);
}

// POST /api/geocoding — the CLI nests the API payload one level deep, and
// index.html reads d.results.results, so keep that shape.
async function handleGeocoding(req) {
  const query = String(req.query || '').trim();
  if (!query) return fail('query is required', 400);

  const params = new URLSearchParams({
    name: query,
    count: '10',
    language: 'en',
    format: 'json',
  });

  const { data, error } = await fetchMeteo(`${GEOCODING_BASE}?${params}`);
  return error || json({ results: data });
}

// POST /api/forecast — current conditions plus the 7-day outlook index.html
// renders and exports.
async function handleForecast(req) {
  const lat = Number(req.latitude);
  const lon = Number(req.longitude);
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
    return fail('latitude and longitude must be numbers', 400);
  }

  const params = new URLSearchParams({
    latitude: lat.toFixed(6),
    longitude: lon.toFixed(6),
    current: 'temperature_2m,apparent_temperature,relative_humidity_2m,precipitation,wind_speed_10m',
    daily: 'temperature_2m_max,temperature_2m_min,precipitation_sum,wind_speed_10m_max',
    forecast_days: '7',
    timezone: 'auto',
    temperature_unit: tempUnit(req.units),
    wind_speed_unit: windUnit(req.units),
    precipitation_unit: precipUnit(req.units),
  });

  const { data, error } = await fetchMeteo(`${FORECAST_BASE}?${params}`);
  return error || json({ results: data });
}

export async function onRequest({ request, params }) {
  if (request.method === 'OPTIONS') return new Response(null, { status: 200, headers: CORS });
  if (request.method !== 'POST') return fail('only POST', 405);

  const route = Array.isArray(params.path) ? params.path.join('/') : String(params.path || '');

  let body;
  try {
    body = await request.json();
  } catch {
    return fail('invalid JSON', 400);
  }
  if (!body || typeof body !== 'object') return fail('invalid JSON', 400);

  switch (route) {
    case 'geocoding':
      return handleGeocoding(body);
    case 'forecast':
      return handleForecast(body);
    case 'history-daily':
      return proxyMeteo(body, ARCHIVE_BASE, 'daily', DAILY_FIELDS, DAILY_DEFAULT);
    case 'history-hourly':
      return proxyMeteo(body, ARCHIVE_BASE, 'hourly', HOURLY_FIELDS, HOURLY_DEFAULT);
    // 15-minute data via the forecast API; Open-Meteo keeps roughly the last 3
    // months of minutely_15 history plus ~16 days of forecast.
    case 'minutely':
    case 'recent-minutely':
      return proxyMeteo(body, FORECAST_BASE, 'minutely_15', MINUTELY_FIELDS, HOURLY_DEFAULT);
    default:
      return fail('not found', 404);
  }
}

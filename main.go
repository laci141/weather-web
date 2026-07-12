package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8097"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/forecast", handleForecast)
	mux.HandleFunc("/api/geocoding", handleGeocoding)
	mux.HandleFunc("/api/history-daily", handleHistoryDaily)
	mux.HandleFunc("/api/history-hourly", handleHistoryHourly)
	mux.HandleFunc("/api/recent-minutely", handleRecentMinutely)
	mux.HandleFunc("/api/minutely", handleRecentMinutely)
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/", handleRoot)

	srv := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("weather-web on 0.0.0.0:%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// --- units ---
// unitSpec arrives from the client and is mapped onto Open-Meteo / CLI unit
// parameters. Every value is validated against an allow-list; anything else
// falls back to the European/metric default.

type unitSpec struct {
	Temperature   string `json:"temperature"`   // celsius | fahrenheit
	Wind          string `json:"wind"`          // kmh | mph
	Precipitation string `json:"precipitation"` // mm | inch
}

func (u unitSpec) temp() string {
	if u.Temperature == "fahrenheit" {
		return "fahrenheit"
	}
	return "celsius"
}

func (u unitSpec) wind() string {
	if u.Wind == "mph" {
		return "mph"
	}
	return "kmh"
}

func (u unitSpec) precip() string {
	if u.Precipitation == "inch" {
		return "inch"
	}
	return "mm"
}

// --- CLI-backed endpoints ---

type forecastRequest struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Units     unitSpec `json:"units"`
}

func handleForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "only POST", http.StatusMethodNotAllowed)
		return
	}

	var req forecastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// The CLI defaults to fahrenheit/mph — always pass explicit units so the
	// European default (celsius/kmh) actually reaches the API.
	cmd := exec.Command(cliBinary(), "forecast",
		"--latitude", fmt.Sprintf("%f", req.Latitude),
		"--longitude", fmt.Sprintf("%f", req.Longitude),
		"--temperature-unit", req.Units.temp(),
		"--wind-speed-unit", req.Units.wind(),
		"--json")
	out, err := cmd.Output()
	if err != nil {
		log.Print(err)
		http.Error(w, "CLI error", http.StatusBadGateway)
		return
	}

	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

type geocodingRequest struct {
	Query string `json:"query"`
}

func handleGeocoding(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "only POST", http.StatusMethodNotAllowed)
		return
	}

	var req geocodingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	cmd := exec.Command(cliBinary(), "geocoding", req.Query, "--json")
	out, err := cmd.Output()
	if err != nil {
		log.Print(err)
		http.Error(w, "CLI error", http.StatusBadGateway)
		return
	}

	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// --- direct Open-Meteo endpoints ---
// The weather-goat CLI's history command builds a malformed URL, so history
// data comes straight from the Open-Meteo APIs with url.Values encoding.

type historyRequest struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Fields    []string `json:"fields"`
	Units     unitSpec `json:"units"`
}

// dateRe strictly validates YYYY-MM-DD before the value goes into an outbound
// URL (defense against injection through the date fields).
var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var meteoClient = &http.Client{Timeout: 60 * time.Second}

// allow-lists: only these field names may reach the upstream query string.
// Expanded to the full set of Open-Meteo variables the UI can request.
var dailyFields = map[string]bool{
	"temperature_2m_max": true, "temperature_2m_min": true, "temperature_2m_mean": true,
	"apparent_temperature_max": true, "apparent_temperature_min": true,
	"relative_humidity_2m_max": true, "relative_humidity_2m_min": true,
	"dew_point_2m_max": true, "dew_point_2m_min": true,
	"precipitation_sum": true, "rain_sum": true, "snowfall_sum": true,
	"surface_pressure_mean": true, "cloud_cover_mean": true,
	"wind_speed_10m_max": true, "wind_gusts_10m_max": true, "wind_direction_10m_dominant": true,
	"sunshine_duration": true, "shortwave_radiation_sum": true,
}

var hourlyFields = map[string]bool{
	"temperature_2m": true, "apparent_temperature": true,
	"relative_humidity_2m": true, "dew_point_2m": true,
	"precipitation": true, "rain": true, "snowfall": true,
	"surface_pressure": true, "cloud_cover": true,
	"wind_speed_10m": true, "wind_gusts_10m": true, "wind_direction_10m": true,
	"sunshine_duration": true, "shortwave_radiation": true,
}

// minutely_15 supports a subset of the hourly variables (no surface_pressure,
// no cloud_cover).
var minutelyFields = map[string]bool{
	"temperature_2m": true, "apparent_temperature": true,
	"relative_humidity_2m": true, "dew_point_2m": true,
	"precipitation": true, "rain": true, "snowfall": true,
	"wind_speed_10m": true, "wind_gusts_10m": true, "wind_direction_10m": true,
	"sunshine_duration": true, "shortwave_radiation": true,
}

// filterFields keeps only allow-listed names; returns def when nothing survives.
func filterFields(req []string, allowed map[string]bool, def string) string {
	var keep []string
	for _, f := range req {
		f = strings.TrimSpace(f)
		if allowed[f] {
			keep = append(keep, f)
		}
	}
	if len(keep) == 0 {
		return def
	}
	return strings.Join(keep, ",")
}

// proxyMeteo validates the request, builds the upstream URL with the given
// resolution parameter (daily / hourly / minutely_15) and unit parameters,
// and streams back JSON.
func proxyMeteo(w http.ResponseWriter, r *http.Request, base, param string, allowed map[string]bool, def string) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "only POST", http.StatusMethodNotAllowed)
		return
	}

	var req historyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if !dateRe.MatchString(req.StartDate) || !dateRe.MatchString(req.EndDate) {
		http.Error(w, "start_date and end_date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", req.Latitude))
	params.Set("longitude", fmt.Sprintf("%f", req.Longitude))
	params.Set("start_date", req.StartDate)
	params.Set("end_date", req.EndDate)
	params.Set(param, filterFields(req.Fields, allowed, def))
	params.Set("timezone", "auto")
	// unit params — European/metric defaults unless the client toggled
	params.Set("temperature_unit", req.Units.temp())
	params.Set("wind_speed_unit", req.Units.wind())
	params.Set("precipitation_unit", req.Units.precip())

	resp, err := meteoClient.Get(base + "?" + params.Encode())
	if err != nil {
		log.Print(err)
		http.Error(w, "weather API unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		http.Error(w, "reading weather API response", http.StatusBadGateway)
		return
	}
	if resp.StatusCode >= 400 {
		http.Error(w, string(body), http.StatusBadGateway)
		return
	}
	// Open-Meteo reports parameter errors as HTTP 200 with {"error":true,...};
	// surface those as 502 so the client shows a clean error instead of
	// treating the payload as data.
	var apiErr struct {
		Error  bool   `json:"error"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error {
		http.Error(w, apiErr.Reason, http.StatusBadGateway)
		return
	}

	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

const archiveBase = "https://archive-api.open-meteo.com/v1/archive"
const forecastBase = "https://api.open-meteo.com/v1/forecast"

// POST /api/history-daily
func handleHistoryDaily(w http.ResponseWriter, r *http.Request) {
	proxyMeteo(w, r, archiveBase, "daily", dailyFields,
		"temperature_2m_max,temperature_2m_min,precipitation_sum,wind_speed_10m_max")
}

// POST /api/history-hourly
func handleHistoryHourly(w http.ResponseWriter, r *http.Request) {
	proxyMeteo(w, r, archiveBase, "hourly", hourlyFields,
		"temperature_2m,relative_humidity_2m,precipitation,wind_speed_10m")
}

// POST /api/minutely (and legacy /api/recent-minutely) — 15-minute data via
// the forecast API; Open-Meteo keeps roughly the last 3 months of minutely_15
// history plus ~16 days of forecast. Client validates the window; upstream
// errors pass through as 502 with the API's message.
func handleRecentMinutely(w http.ResponseWriter, r *http.Request) {
	proxyMeteo(w, r, forecastBase, "minutely_15", minutelyFields,
		"temperature_2m,relative_humidity_2m,precipitation,wind_speed_10m")
}

func cliBinary() string {
	if b := os.Getenv("CLI_BIN"); b != "" {
		return b
	}
	return "weather-goat-pp-cli"
}

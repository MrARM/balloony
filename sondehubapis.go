package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SHPredictionResult struct {
	Vehicle       string    `json:"vehicle"`
	Time          time.Time `json:"time"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Altitude      float64   `json:"altitude"`
	AscentRate    float64   `json:"ascent_rate"`
	DescentRate   float64   `json:"descent_rate"`
	BurstAltitude float64   `json:"burst_altitude"`
	Descending    int       `json:"descending"`
	Landed        int       `json:"landed"`
	Data          string    `json:"data"`
}

// GetPrediction fetches the first SHPredictionResult for a given serial from SondeHub API.
func GetPrediction(serial string) (*SHPredictionResult, error) {
	url := fmt.Sprintf("https://api.v2.sondehub.org/predictions?vehicles=%s", serial)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prediction: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sondehub API returned status: %s", resp.Status)
	}
	var results []SHPredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode prediction response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no prediction results found for serial: %s", serial)
	}
	pred := &results[0]
	// Decode the JSON-in-JSON data field
	var predData []struct {
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Time float64 `json:"time"`
	}
	if err := json.Unmarshal([]byte(pred.Data), &predData); err != nil {
		return nil, fmt.Errorf("failed to decode prediction data field: %w", err)
	}
	if len(predData) == 0 {
		return nil, fmt.Errorf("no prediction data points found in data field")
	}
	last := predData[len(predData)-1]
	pred.Latitude = last.Lat
	pred.Longitude = last.Lon
	pred.Time = time.Unix(int64(last.Time), 0).UTC()
	return pred, nil
}

// GetReceivers fetches receiver locations from SondeHub and returns a []Point (lat/lon/name)
func GetReceivers() ([]Point, error) {
	resp, err := http.Get("https://api.v2.sondehub.org/listeners/telemetry")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch receivers: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sondehub API returned status: %s", resp.Status)
	}
	var raw map[string]map[string]struct {
		UploaderPosition []float64 `json:"uploader_position"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode receivers: %w", err)
	}
	points := make([]Point, 0, len(raw))
	for name, times := range raw {
		for _, v := range times { // Only need the first timestamp entry
			if len(v.UploaderPosition) >= 2 {
				points = append(points, Point{
					Lat:  v.UploaderPosition[0],
					Lon:  v.UploaderPosition[1],
					Name: name,
				})
			}
			break // Only use the first timestamp
		}
	}
	return points, nil
}

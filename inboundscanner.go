package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	inboundScanInterval  = 15 * time.Minute
	inboundScanRadiusMi  = 200.0 // miles outside the boundary to look for sondes
)

// startInboundScanner polls SondeHub every 15 minutes for sondes that are currently
// outside ALERT_BOUNDS but have a predicted landing inside it, and pre-populates
// recentlySeen so they are tracked as soon as the next MQTT packet arrives.
func startInboundScanner() {
	go func() {
		for {
			scanInbound()
			time.Sleep(inboundScanInterval)
		}
	}()
}

func scanInbound() {
	// Build a conservative search radius: max distance from boundary centroid to any
	// vertex, plus the 200-mile buffer.
	cLon, cLat := polyCentroid(boundaryPts)
	searchRadius := polyMaxRadius(boundaryPts, cLon, cLat) + inboundScanRadiusMi

	resp, err := http.Get("https://api.v2.sondehub.org/predictions")
	if err != nil {
		fmt.Println("Inbound scanner: error fetching predictions:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("Inbound scanner: predictions API returned %s\n", resp.Status)
		return
	}

	var preds []SHPredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&preds); err != nil {
		fmt.Println("Inbound scanner: error decoding predictions:", err)
		return
	}

	added := 0
	for i := range preds {
		serial := preds[i].Vehicle

		// Skip serials already being tracked.
		recentlySeenMu.RLock()
		_, known := recentlySeen[serial]
		recentlySeenMu.RUnlock()
		if known {
			continue
		}

		// Decode the JSON-in-JSON Data field; first point = current, last = landing.
		if preds[i].Data == "" {
			continue
		}
		var track []struct {
			Lat  float64 `json:"lat"`
			Lon  float64 `json:"lon"`
			Time float64 `json:"time"`
		}
		if err := json.Unmarshal([]byte(preds[i].Data), &track); err != nil || len(track) == 0 {
			continue
		}
		cur := track[0]
		land := track[len(track)-1]

		// Gate 1: sonde must be within the search radius of the boundary centroid.
		if haversineMiles(cLat, cLon, cur.Lat, cur.Lon) > searchRadius {
			continue
		}

		// Gate 2: skip sondes already inside the boundary (handled by normal MQTT path).
		if InsidePoly([]float64{cur.Lon, cur.Lat}, boundaryPts) {
			continue
		}

		// Gate 3: predicted landing must be inside the boundary.
		if !InsidePoly([]float64{land.Lon, land.Lat}, boundaryPts) {
			continue
		}

		// Pre-populate recentlySeen; the next MQTT packet for this serial will
		// flow through handleNewSonde and trigger the Discord alert.
		recentlySeenMu.Lock()
		recentlySeen[serial] = time.Now()
		recentlySeenMu.Unlock()

		fmt.Printf("Inbound scanner: pre-tracking %s — predicted landing at %.4f, %.4f\n",
			serial, land.Lat, land.Lon)
		added++
	}

	if added > 0 {
		fmt.Printf("Inbound scanner: added %d inbound sonde(s) to tracking\n", added)
	}
}

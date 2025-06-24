package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"time"
)

// Point struct for coordinates and name
// This is now the canonical definition for the project
// Used for launch sites, receivers, etc.
type Point struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Name string  `json:"name"`
}

// InsidePoly returns true if the point is inside the polygon defined by vs (ray-casting algorithm)
func InsidePoly(point []float64, vs [][]float64) bool {
	x, y := point[0], point[1]
	inside := false
	for i, j := 0, len(vs)-1; i < len(vs); j, i = i, i+1 {
		xi, yi := vs[i][0], vs[i][1]
		xj, yj := vs[j][0], vs[j][1]
		if ((yi > y) != (yj > y)) && (x < (xj-xi)*(y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
	}
	return inside
}

// This handles finding the nearest point in a radial search

// Haversine formula to calculate distance between two lat/lon points in miles
func haversineMiles(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 3958.8 // Earth radius in miles
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// FindClosestPoint returns the closest Point and its distance (in miles) to the given lat/lon
func FindClosestPoint(lat, lon float64, points []Point) (Point, float64, error) {
	if len(points) == 0 {
		return Point{}, 0, errors.New("no points provided")
	}
	minDist := math.MaxFloat64
	var closest Point
	for _, p := range points {
		dist := haversineMiles(lat, lon, p.Lat, p.Lon)
		if dist < minDist {
			minDist = dist
			closest = p
		}
	}
	return closest, minDist, nil
}

// ParseLaunchSitesJSON parses launchsites.json into a []Point
func ParseLaunchSitesJSON(filename string) ([]Point, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// The JSON is a map of string to struct with position and station_name
	var raw map[string]struct {
		Position    []float64 `json:"position"`
		StationName string    `json:"station_name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	points := make([]Point, 0, len(raw))
	for _, v := range raw {
		if len(v.Position) == 2 {
			points = append(points, Point{
				Lat:  v.Position[1], // Note: [lon, lat] or [lat, lon]? Check JSON order
				Lon:  v.Position[0],
				Name: v.StationName,
			})
		}
	}
	return points, nil
}

// Meters to Feet conversion
func MetersToFeet(meters float64) float64 {
	const feetPerMeter = 3.28084
	return meters * feetPerMeter
}

// Get Location from Radar response (City, ST or <County> County, ST)
func GetLocationFromRadarResponse(resp RadarGeoResponse) string {
	// Since the city can be empty, we can also use the county
	// Keep in mind that this was designed for US locations, but it wouldn't be hard to extend it to other countries
	var locationText string
	// Just in case, if the response is empty, return a default message
	if len(resp.Addresses) == 0 {
		return "Location not found"
	}
	if resp.Addresses[0].City != "" {
		locationText = fmt.Sprintf("%s, %s", resp.Addresses[0].City, resp.Addresses[0].StateCode)
	} else if resp.Addresses[0].County != "" {
		locationText = fmt.Sprintf("%s County, %s", resp.Addresses[0].County, resp.Addresses[0].StateCode)
	}

	return locationText
}

// IsUsualTime returns true if the UTC hour is 11-13 or 23-01
func IsUsualTime(t time.Time) bool {
	hour := t.UTC().Hour()
	if (hour >= 11 && hour <= 13) || hour == 23 || hour == 0 || hour == 1 {
		return true
	}
	return false
}

// defaultString returns def if val is empty, otherwise returns val
func defaultString(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

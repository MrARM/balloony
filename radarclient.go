package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type RadarGeoResponse struct {
	Meta struct {
		Code int `json:"code"`
	} `json:"meta"`
	Addresses []struct {
		AddressLabel     string  `json:"addressLabel"`
		Number           string  `json:"number"`
		Street           string  `json:"street"`
		City             string  `json:"city"`
		State            string  `json:"state"`
		StateCode        string  `json:"stateCode"`
		PostalCode       string  `json:"postalCode"`
		County           string  `json:"county"`
		CountryCode      string  `json:"countryCode"`
		FormattedAddress string  `json:"formattedAddress"`
		Layer            string  `json:"layer"`
		Latitude         float64 `json:"latitude"`
		Longitude        float64 `json:"longitude"`
		Geometry         struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Distance    float64 `json:"distance"`
		Country     string  `json:"country"`
		CountryFlag string  `json:"countryFlag"`
		TimeZone    struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Code        string `json:"code"`
			CurrentTime string `json:"currentTime"`
			UtcOffset   int    `json:"utcOffset"`
			DstOffset   int    `json:"dstOffset"`
		} `json:"timeZone"`
	} `json:"addresses"`
}

type RadarDistanceResponse struct {
	Meta struct {
		Code int `json:"code"`
	} `json:"meta"`
	Routes struct {
		Geodesic struct {
			Distance struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"distance"`
		} `json:"geodesic"`
		Car struct {
			Duration struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"duration"`
			Distance struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"distance"`
		} `json:"car"`
	} `json:"routes"`
}

// RadarReverseGeocode calls Radar.io reverse geocode API and returns RadarGeoResponse
func RadarReverseGeocode(lat, lon float64) (RadarGeoResponse, error) {
	var respObj RadarGeoResponse
	apiKey := os.Getenv("RADAR_API_KEY")
	if apiKey == "" {
		return respObj, fmt.Errorf("RADAR_API_KEY not set in environment")
	}
	url := fmt.Sprintf("https://api.radar.io/v1/geocode/reverse?coordinates=%f,%f&layers=", lat, lon)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return respObj, err
	}
	req.Header.Set("Authorization", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return respObj, fmt.Errorf("radar API error: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return respObj, err
	}
	if err := json.Unmarshal(body, &respObj); err != nil {
		return respObj, err
	}
	return respObj, nil
}

// RadarDistanceCalc calls Radar.io distance API and returns RadarDistanceResponse
func RadarDistanceCalc(origin, destination []float64) (RadarDistanceResponse, error) {
	var respObj RadarDistanceResponse
	if len(origin) != 2 || len(destination) != 2 {
		return respObj, fmt.Errorf("origin and destination must be [lat,lon]")
	}
	apiKey := os.Getenv("RADAR_API_KEY")
	if apiKey == "" {
		return respObj, fmt.Errorf("RADAR_API_KEY not set in environment")
	}
	url := fmt.Sprintf("https://api.radar.io/v1/route/distance?origin=%f,%f&destination=%f,%f&modes=car&units=imperial", origin[0], origin[1], destination[0], destination[1])
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return respObj, err
	}
	req.Header.Set("Authorization", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return respObj, fmt.Errorf("radar API error: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return respObj, err
	}
	if err := json.Unmarshal(body, &respObj); err != nil {
		return respObj, err
	}
	return respObj, nil
}
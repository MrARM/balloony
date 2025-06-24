package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type SHPacket struct {
	SoftwareName     string    `json:"software_name"`
	SoftwareVersion  string    `json:"software_version"`
	UploaderCallsign string    `json:"uploader_callsign"`
	UploaderPosition string    `json:"uploader_position"`
	UploaderAntenna  string    `json:"uploader_antenna"`
	TimeReceived     time.Time `json:"time_received"`
	Datetime         time.Time `json:"datetime"`
	Manufacturer     string    `json:"manufacturer"`
	Type             string    `json:"type"`
	Serial           string    `json:"serial"`
	Subtype          string    `json:"subtype"`
	Frame            int       `json:"frame"`
	Lat              float64   `json:"lat"`
	Lon              float64   `json:"lon"`
	Alt              float64   `json:"alt"`
	Temp             float64   `json:"temp"`
	Humidity         float64   `json:"humidity"`
	VelV             float64   `json:"vel_v"`
	VelH             float64   `json:"vel_h"`
	Heading          float64   `json:"heading"`
	Sats             int       `json:"sats"`
	Batt             float64   `json:"batt"`
	Frequency        float64   `json:"frequency"`
	BurstTimer       int       `json:"burst_timer"`
	RefPosition      string    `json:"ref_position"`
	RefDatetime      string    `json:"ref_datetime"`
	Rs41Mainboard    string    `json:"rs41_mainboard"`
	Rs41MainboardFw  string    `json:"rs41_mainboard_fw"`
	Snr              float64   `json:"snr"`
	TxFrequency      float64   `json:"tx_frequency"`
	UserAgent        string    `json:"user-agent"`
	Position         string    `json:"position"`
	UploadTimeDelta  float64   `json:"upload_time_delta"`
	UploaderAlt      float64   `json:"uploader_alt"`
}

// FilterUnique returns a map of serial -> SHPacket, keeping only the packet with the highest Frame for each serial.
func FilterUnique(packets []SHPacket) map[string]SHPacket {
	result := make(map[string]SHPacket)
	for _, pkt := range packets {
		existing, found := result[pkt.Serial]
		if !found || pkt.Frame > existing.Frame {
			result[pkt.Serial] = pkt
		}
	}
	return result
}

// ParseBatch takes in a JSON array of SHPackets and returns the filtered unique packets.
func ParseBatch(data []byte) ([]SHPacket, error) {
	var packets []SHPacket
	if err := json.Unmarshal(data, &packets); err != nil {
		return nil, err
	}

	uniquePackets := FilterUnique(packets)
	result := make([]SHPacket, 0, len(uniquePackets))
	for _, pkt := range uniquePackets {
		result = append(result, pkt)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no packets found in JSON") // Return error if no packets found
	}

	return result, nil
}

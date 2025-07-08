package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"time"

	staticmaps "github.com/flopp/go-staticmaps"
	"github.com/golang/geo/s2"
)

// RenderSondeMap renders a map with the radiosonde position, predicted path, and landing point.
// It saves the PNG to the current directory with a timestamped filename.
func RenderSondeMap(pkt SHPacket, shPred *SHPredictionResult) (string, error) {
	m := staticmaps.NewContext()
	m.SetSize(1280, 720)
	m.SetTileProvider(staticmaps.NewTileProviderOpenStreetMaps())
	m.SetMaxZoom(19) // Fixes Issue #8 - Map does not draw tiles at low altitudes

	// Determine cache directory from env or default
	cacheDir := os.Getenv("TILE_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "tilecache"
	}
	m.SetCache(staticmaps.NewTileCache(cacheDir, 0o755))

	// Attribution for icons
	// Balloon and target icons © Rossen Georgiev, MIT License, https://github.com/projecthorus/sondehub-tracker
	// NOTE: Convert assets/balloon.svg and assets/target.svg to assets/balloon.png and assets/target.png for best results.

	balloonImgPath := "assets/balloon.png"
	if pkt.VelV < 0 {
		balloonImgPath = "assets/parachute.png"
	}
	balloonImg, _ := loadPNGAsImage(balloonImgPath)
	targetImg, _ := loadPNGAsImage("assets/target.png")

	addTargetMarker(m, shPred, targetImg)

	balloonRendered, err := renderPathAndBalloon(m, shPred, balloonImg)
	if err != nil {
		fmt.Println("[WARN] Could not parse shPred.Data as objects:", err)
		fmt.Println("Raw shPred.Data:", shPred.Data)
	}
	if !balloonRendered {
		addBalloonFallback(m, pkt, balloonImg)
	}

	m.OverrideAttribution(fmt.Sprintf("Balloony - Tracking %s %s on %s - Thanks to OpenStreetMap contributors and SondeHub!", pkt.Type, pkt.Serial, pkt.Datetime.Format("01/02/2006")))

	// Ensure render_debug directory exists
	outputDir := "render_debug"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	img, err := m.Render()
	if err != nil {
		return "", fmt.Errorf("map render error: %w", err)
	}

	filename := filepath.Join(outputDir, "sonde_map_"+strconv.FormatInt(time.Now().Unix(), 10)+".png")
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("file create error: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", fmt.Errorf("png encode error: %w", err)
	}
	return filename, nil
}

// RenderSondeMapToBuffer renders the map and returns the PNG as a bytes.Buffer (in-memory)
func RenderSondeMapToBuffer(pkt SHPacket, shPred *SHPredictionResult) (*bytes.Buffer, error) {
	m := staticmaps.NewContext()
	m.SetSize(1280, 720)
	m.SetTileProvider(staticmaps.NewTileProviderOpenStreetMaps())

	cacheDir := os.Getenv("TILE_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "tilecache"
	}
	m.SetCache(staticmaps.NewTileCache(cacheDir, 0o755))

	balloonImgPath := "assets/balloon.png"
	if pkt.VelV < 0 {
		balloonImgPath = "assets/parachute.png"
	}
	balloonImg, _ := loadPNGAsImage(balloonImgPath)
	targetImg, _ := loadPNGAsImage("assets/target.png")

	addTargetMarker(m, shPred, targetImg)
	balloonRendered, err := renderPathAndBalloon(m, shPred, balloonImg)
	if err != nil {
		fmt.Println("[WARN] Could not parse shPred.Data as objects:", err)
		fmt.Println("Raw shPred.Data:", shPred.Data)
	}
	if !balloonRendered {
		addBalloonFallback(m, pkt, balloonImg)
	}

	m.OverrideAttribution(fmt.Sprintf("Balloony - Tracking %s %s on %s (UTC) - Thanks to OpenStreetMap contributors and SondeHub!", pkt.Type, pkt.Serial, pkt.Datetime.Format("01/02/2006 15:04:05")))

	img, err := m.Render()
	if err != nil {
		return nil, fmt.Errorf("map render error: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return nil, fmt.Errorf("png encode error: %w", err)
	}
	return buf, nil
}

// Helper: Add target marker or image
func addTargetMarker(m *staticmaps.Context, shPred *SHPredictionResult, targetImg image.Image) {
	if targetImg != nil {
		m.AddObject(staticmaps.NewImageMarker(
			s2.LatLngFromDegrees(shPred.Latitude, shPred.Longitude),
			targetImg,
			float64(targetImg.Bounds().Dx()/2),
			float64(targetImg.Bounds().Dy()/2),
		))
	} else {
		m.AddMarker(&staticmaps.Marker{
			Position: s2.LatLngFromDegrees(shPred.Latitude, shPred.Longitude),
			Color:    color.RGBA{R: 0, G: 200, B: 0, A: 255},
			Size:     16,
		})
	}
}

// Helper: Parse and render path, and add balloon image at start
func renderPathAndBalloon(m *staticmaps.Context, shPred *SHPredictionResult, balloonImg image.Image) (bool, error) {
	type PathPoint struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	var pathObjs []PathPoint
	if err := json.Unmarshal([]byte(shPred.Data), &pathObjs); err != nil {
		return false, err
	}
	if len(pathObjs) > 1 {
		var polyline []s2.LatLng
		for _, pt := range pathObjs {
			polyline = append(polyline, s2.LatLngFromDegrees(pt.Lat, pt.Lon))
		}
		m.AddObject(staticmaps.NewPath(polyline, color.RGBA{R: 255, A: 255}, 4))
		if balloonImg != nil {
			first := pathObjs[0]
			imgW := float64(balloonImg.Bounds().Dx())
			imgH := float64(balloonImg.Bounds().Dy())
			m.AddObject(staticmaps.NewImageMarker(
				s2.LatLngFromDegrees(first.Lat, first.Lon),
				balloonImg,
				imgW/2, imgH-12,
			))
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// Helper: Fallback for balloon at radiosonde position
func addBalloonFallback(m *staticmaps.Context, pkt SHPacket, balloonImg image.Image) {
	if balloonImg != nil {
		imgW := float64(balloonImg.Bounds().Dx())
		imgH := float64(balloonImg.Bounds().Dy())
		m.AddObject(staticmaps.NewImageMarker(s2.LatLngFromDegrees(pkt.Lat, pkt.Lon), balloonImg, imgW/2, imgH+12))
	} else {
		m.AddMarker(&staticmaps.Marker{
			Position: s2.LatLngFromDegrees(pkt.Lat, pkt.Lon),
			Color:    color.RGBA{R: 255, G: 0, B: 0, A: 255},
			Size:     16,
		})
	}
}

// loadPNGAsImage loads a PNG file as image.Image
func loadPNGAsImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

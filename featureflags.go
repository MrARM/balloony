package main

import "sync"

// FeatureFlags holds in-memory runtime feature flag state.
// All fields are reset to defaults on restart.
type FeatureFlags struct {
	// ThunderforestEnabled replaces the standard OSM map tiles with Thunderforest tiles.
	ThunderforestEnabled bool
	// ThunderforestStyle is the Thunderforest map style (e.g. "cycle", "spinal-map").
	ThunderforestStyle string
	// ThunderforestAPIKey is the Thunderforest API key.
	ThunderforestAPIKey string
}

var featureFlags = FeatureFlags{
	ThunderforestStyle: "cycle",
}
var featureFlagsMu sync.RWMutex

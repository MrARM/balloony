package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	dotenv "github.com/joho/godotenv"
)

// Development hard-coded bypass to the location filtering
// Warning: This could get messy if ran during normal launch hours
var bypassLocationFilter = false

// Variables we keep in-memory
var boundaryPts [][]float64
var launchSites []Point
var redisclient *RedisMgr
var updateInterval int64
var timezone string = "Etc/UTC"
var message_usual = "A new sonde has been detected!"
var message_unusual = "Unusual Sonde Detected!"

var receivers []Point
var receiversMutex sync.RWMutex

const defaultReceiversUpdateInterval = 12 * 60 * 60 // 12 hours in seconds
const zeroWidthSpace = "\u200B"

// Routine to periodically update the receivers list every 12 hours
func startReceiversUpdater() {
	go func() {
		for {
			updated, err := GetReceivers()
			if err != nil {
				fmt.Println("Error updating receivers:", err)
			} else {
				receiversMutex.Lock()
				receivers = updated
				receiversMutex.Unlock()
				fmt.Printf("Receivers list updated: %d receivers loaded\n", len(receivers))
			}
			time.Sleep(time.Duration(defaultReceiversUpdateInterval) * time.Second)
		}
	}()
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// Parse the message payload into a SondeHub packet
	pkts, err := ParseBatch(msg.Payload())
	if err != nil {
		fmt.Println("Error parsing packets:", err)
		return
	}

	// In most situations, we only get 1 packet, but we still handle it with a foreach in the situation where we have a multi-sdr receiver
	for _, pkt := range pkts {
		// TEST: Test the nearest point functionality
		// closest, dist, err := FindClosestPoint(pkt.Lat, pkt.Lon, launchSites)
		// if err != nil {
		// 	fmt.Println("Error finding closest point:", err)
		// 	return
		// }
		// fmt.Printf("Closest launch site to %s is %s at distance %.2f miles\n", pkt.Serial, closest.Name, dist)

		// This is the main processing loop for incoming sondehub packets
		// Check to see if the sonde is inside of our area of interest
		if !InsidePoly([]float64{pkt.Lon, pkt.Lat}, boundaryPts) {
			// Skip packets that are outside the defined boundary
			if !bypassLocationFilter {
				return
			}
		}

		if !claimSonde(pkt.Serial) {
			continue
		}

		// Then check to see if we have a new sonde or an existing sonde
		session, err := redisclient.GetSondeSession(pkt.Serial)
		if err != nil {
			fmt.Println("Error getting SondeSession from Redis:", err)
			releaseSonde(pkt.Serial)
			return
		}

		// iMet sonde VelV spoofing
		if pkt.Manufacturer == "Intermet Systems" {
			if session != nil {
				fmt.Println("Existing iMetAlt:", session.IMetAlt)
				// If IMetAlt is set(we use omitempty)
				if session.IMetAlt != 0 {
					if pkt.Alt < float64(session.IMetAlt) {
						// if the altitude has dropped, set the VelV to -1
						pkt.VelV = -1
					} else {
						// If the altitude is higher than the IMetAlt, we set it to 1
						pkt.VelV = 1
					}
				}
				// set the IMetAlt to the current altitude
				session.IMetAlt = int(pkt.Alt)
			}
		}

		if session == nil {
			handleNewSonde(pkt)
		} else {
			handleSonde(pkt, session)
		}

		releaseSonde(pkt.Serial)
	}

}

func handleSonde(pkt SHPacket, session *SondeSession) {
	// Check to see if the session time has been long enough
	if pkt.TimeReceived.Unix() < session.Time+updateInterval {
		// Conditionally, if the sonde is descending and less than 10kft,
		// our update interval changes to 30 seconds
		if pkt.Alt < 3048 && pkt.VelV < 0 { // 10,000 feet in meters
			if pkt.TimeReceived.Unix() < session.Time+30 {
				// If the packet is less than 30 seconds old, we don't update
				return
			}
		} else {
			// High Altitude + not passed interval
			return
		}
	}

	// Pull Geo APIs for reverse geocoding
	actLoc, err := RadarReverseGeocode(pkt.Lat, pkt.Lon)
	if err != nil {
		fmt.Println("Error reverse geocoding:", err)
		return
	}

	shPred, err := GetPrediction(pkt.Serial)
	if err != nil {
		fmt.Println("Error getting prediction:", err)
		return
	}

	// Render the map image to memory for Discord upload
	hasImage := false
	buf, err := RenderSondeMapToBuffer(pkt, shPred)
	if err != nil {
		fmt.Println("Error rendering map image:", err)
	} else {
		hasImage = true
	}

	predLoc, err := RadarReverseGeocode(shPred.Latitude, shPred.Longitude)
	if err != nil {
		fmt.Println("Error reverse geocoding prediction:", err)
		return
	}

	// Build the message to send to Discord
	var fields []DiscordField
	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Frequency: %.1f MHz", pkt.Frequency),
		Value: zeroWidthSpace,
	})

	// We add a unicode arrow pointer to show if it's ascending or descending
	var arrow string = "\u2193"
	if pkt.VelV > 0 {
		arrow = "\u2191"
	}
	if pkt.VelV == 0 {
		arrow = "--"
	}

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Altitude: %s ft %s", humanize.Comma(int64(MetersToFeet(pkt.Alt))), arrow),
		Value: zeroWidthSpace,
	})

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Over %s", GetLocationFromRadarResponse(actLoc)),
		Value: zeroWidthSpace,
	})

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading timezone:", err)
		loc = time.UTC
	}

	localPredTime := shPred.Time.In(loc)
	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Predicted to land in %s around %s", GetLocationFromRadarResponse(predLoc), localPredTime.Format("3:04 PM")),
		Value: zeroWidthSpace,
	})

	// Check to see if anybody is nearby (20 miles)
	receiversMutex.RLock()
	point, dist, recerr := FindClosestPoint(shPred.Latitude, shPred.Longitude, receivers)
	receiversMutex.RUnlock()
	if recerr != nil {
		fmt.Println("Error finding closest receiver:", recerr)
	}
	// Assuming Point has a Name field that is non-empty for valid points
	if point.Name != "" {
		// If the closest receiver is within 20 miles, add it to the fields
		if dist < 20 {
			fields = append(fields, DiscordField{
				Name:  fmt.Sprintf("Landing nearby **%s** (%.1f mi)", point.Name, dist),
				Value: zeroWidthSpace,
			})
		}
		// else {
		// 	fields = append(fields, DiscordField{
		// 		Name:  fmt.Sprintf("Closest receiver: %s at %.2f miles (too far away)", point.Name, dist),
		// 		Value: zeroWidthSpace,
		// 	})
		// }
	}

	// If RS41, add the RS41 date of manufacture
	if pkt.Type == "RS41" {
		rstime, err := ResolveRS41Date(pkt.Serial)
		if err == nil {
			fields = append(fields, DiscordField{
				Name:  fmt.Sprintf("Sonde Manufactured: %s", rstime.Format("1/2/2006")),
				Value: zeroWidthSpace,
			})
		} else {
			fmt.Println("Error resolving RS41 date:", err)
		}
	}

	embedTitle := fmt.Sprintf("%s %s is airborne", defaultString(pkt.Subtype, pkt.Type), pkt.Serial)

	embed := DiscordEmbed{
		Type:        "rich",
		Title:       embedTitle,
		Description: "",
		Color:       0x00FFFF,
		Url:         fmt.Sprintf("https://sondehub.org/%s", pkt.Serial),
		Fields:      fields,
	}

	if hasImage {
		SendUpdatedWebhookWithImage(session.Webhook, &embed, buf)
	} else {
		// Since we are updating an existing message, we don't need to send a content field
		message := DiscordMessage{
			Embeds: []DiscordEmbed{embed},
		}

		_, derr := SendDiscordWebhook(message, session.Webhook, true)
		if derr != nil {
			fmt.Println("Error sending Discord message:", derr)
			return
		}
	}

	// Update the time in the session
	session.Time = pkt.TimeReceived.Unix()

	err = redisclient.SaveSondeSession(pkt.Serial, session)
	if err != nil {
		fmt.Println("Error saving SondeSession to Redis:", err)
		return
	}
}

func handleNewSonde(pkt SHPacket) {
	// This function handles new sondes that are detected
	fmt.Printf("New sonde detected: %s at %s\n", pkt.Serial, pkt.TimeReceived) // placeholder
	session := &SondeSession{
		Time:     pkt.TimeReceived.Unix(),
		Webhook:  os.Getenv("DISCORD_WEBHOOK_URL"),
		FromText: "",
	}

	// Attempt to find out where the sonde was launched from
	closest, dist, err := FindClosestPoint(pkt.Lat, pkt.Lon, launchSites)
	if err != nil {
		fmt.Println("Error finding closest launch site:", err)
		session.FromText = ""
	}
	if dist < 10 { // If the closest launch site is within 10 miles
		session.FromText = fmt.Sprintf("From %s", closest.Name)
	}

	// Then get the sonde's reverse geocode location
	loc, err := RadarReverseGeocode(pkt.Lat, pkt.Lon)
	if err != nil {
		fmt.Println("Error reverse geocoding:", err)
	}

	// Then build a message to send to Discord
	var fields []DiscordField

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Frequency: %.1f MHz", pkt.Frequency),
		Value: zeroWidthSpace,
	})

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Launched from %s", GetLocationFromRadarResponse(loc)),
		Value: zeroWidthSpace,
	})

	fields = append(fields, DiscordField{
		Name: fmt.Sprintf("Altitude: %s ft", humanize.Comma(int64(MetersToFeet(pkt.Alt)))),

		Value: zeroWidthSpace,
	})

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("First detected by: %s", pkt.UploaderCallsign),
		Value: zeroWidthSpace,
	})

	fields = append(fields, DiscordField{
		Name:  fmt.Sprintf("Prediction available in <t:%d:R>", pkt.TimeReceived.Unix()+updateInterval),
		Value: zeroWidthSpace,
	})

	// Generate the strings that are conditional
	usualTime := IsUsualTime(pkt.TimeReceived)
	var messageContent *string
	embedTitle := fmt.Sprintf("%s %s is airborne %s", defaultString(pkt.Subtype, pkt.Type), pkt.Serial, session.FromText)
	if usualTime {
		messageContent = &message_usual
	} else {
		messageContent = &message_unusual
	}

	embed := DiscordEmbed{
		Type:        "rich",
		Title:       embedTitle,
		Description: "",
		Color:       0x00FFFF,
		Url:         fmt.Sprintf("https://sondehub.org/%s", pkt.Serial),
		Fields:      fields,
	}

	message := DiscordMessage{
		Content: *messageContent,
		Embeds:  []DiscordEmbed{embed},
	}

	// Send the message to Discord
	res, err := SendDiscordWebhook(message, session.Webhook, false)
	if err != nil {
		fmt.Println("Error sending Discord message:", err)
		return
	}

	// Update the webhook URL in the session
	session.Webhook = fmt.Sprintf("%s/messages/%s", session.Webhook, res.ID)
	// Save the session to Redis
	err = redisclient.SaveSondeSession(pkt.Serial, session)
	if err != nil {
		fmt.Println("Error saving SondeSession to Redis:", err)
		return
	}
}

func main() {
	// Check for required environment variables
	err := dotenv.Load()
	requiredVars := []string{"RADAR_API_KEY", "ALERT_BOUNDS", "DISCORD_WEBHOOK_URL", "UPDATE_INTERVAL"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			log.Fatalf("Required environment variable %s is not set", v)
			panic(fmt.Sprintf("Required environment variable %s is not set", v))
		}
	}

	// Load the boundary points
	bounds := os.Getenv("ALERT_BOUNDS")
	if err := json.Unmarshal([]byte(bounds), &boundaryPts); err != nil {
		log.Fatalf("Error parsing ALERT_BOUNDS: %v", err)
		panic(fmt.Sprintf("Error parsing ALERT_BOUNDS: %v", err))
	}

	// If we have a set timezone, use it
	if tz := os.Getenv("TIMEZONE"); tz != "" {
		timezone = tz
	}

	// Check to see if we have custom messages defined
	if msg := os.Getenv("MESSAGE_USUAL"); msg != "" {
		message_usual = msg
	}

	if msg := os.Getenv("MESSAGE_UNUSUAL"); msg != "" {
		message_unusual = msg
	}

	// Load the update interval
	updateIntervalStr := os.Getenv("UPDATE_INTERVAL")
	updateInterval, err = strconv.ParseInt(updateIntervalStr, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing UPDATE_INTERVAL: %v", err)
		panic(fmt.Sprintf("Error parsing UPDATE_INTERVAL: %v", err))
	}

	// Load the launch sites from launchsites.json (should be in the same directory)
	launchSites, err = ParseLaunchSitesJSON("launchsites.json")
	if err != nil {
		log.Fatalf("Error loading launch sites: %v", err)
		panic(fmt.Sprintf("Error loading launch sites: %v", err))
	}

	// Check for bypassLocationFilter environment variable
	if bypassEnv := os.Getenv("BYPASS_LOCATION_FILTER"); bypassEnv != "" {
		if bypassEnv == "true" || bypassEnv == "1" {
			bypassLocationFilter = true
			fmt.Println("Bypass location filter is enabled. All sondes will be processed regardless of location.")
		} else {
			bypassLocationFilter = false
		}
	}

	// Connect to sondehub MQTT broker
	var broker = "ws-reader.v2.sondehub.org"
	var port = 443

	mqttclient := MQTTConnection(broker, port, "balloonyv2", messagePubHandler)
	if token := mqttclient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Error connecting to SondeHub", token.Error())
		panic(token.Error())
	}

	redisclient = NewRedisClient()
	err = redisclient.Ping()
	if err != nil {
		fmt.Println("Error connecting to Redis:", err)
		panic(err)
	}

	// Start the receivers updater goroutine
	startReceiversUpdater()

	// Wait for Ctrl+C (SIGINT) to exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nExiting...")
}

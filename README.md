# Balloony

A Discord Webhook-based bot that provides real-time alerts and updates for Radiosonde flights using the SondeHub API.

---

## Table of Contents

- [Environment Variables](#environment-variables)
- [System Pipeline](#system-pipeline)
- [Setup Instructions](#setup-instructions)
- [Use in areas outside of the United States](#use-in-areas-outside-of-the-united-states)
- [Bundled Launch Site JSON](#bundled-launch-site-json)
- [License](#license)

---

## Environment Variables

| Name                       | Required | Description                                                                                                 |
|----------------------------|:--------:|-------------------------------------------------------------------------------------------------------------|
| `RADAR_API_KEY`            |   Yes    | API key for Radar.com reverse geocoding. [See below](#radarcom-api-key)                                     |
| `ALERT_BOUNDS`             |   Yes    | JSON array of boundary points (see [Alert Boundaries Format](#alert-boundaries-format))                     |
| `DISCORD_WEBHOOK_URL`      |   Yes    | Discord webhook URL for sending alerts                                                                      |
| `UPDATE_INTERVAL`          |   Yes    | Interval (in seconds) between updates for each sonde                                                        |
| `TIMEZONE`                 |    No    | Timezone for displaying times (default: `Etc/UTC`)                                                          |
| `MESSAGE_USUAL`            |    No    | Custom message for usual launches (default: "A new sonde has been detected!")                               |
| `MESSAGE_UNUSUAL`          |    No    | Custom message for unusual launches (default: "Unusual Sonde Detected!")                                    |
| `TILE_CACHE_DIR`           |    No    | Location to store OSM tiles. Defaults to `./tilecache`                                                      |
| `MAP_SATELLITE_ALTITUDE_FT`|    No    | Threshold to switch to ArcGIS satellite maps for landing location (ft). Default: 10,000 ft.                 |
| `REDIS_ADDR`               |    No    | Redis server address (default: `localhost:6379`)                                                            |
| `REDIS_DB`                 |    No    | Redis database index if required, defaults to 0.                                                            |
| `REDIS_PASSWORD`           |    No    | Redis password if required. Blank by default.                                                               |

---

## System Pipeline

**Startup**: Loads environment variables and parses the alert boundary and launch sites, then Connects to SondeHub MQTT and subscribes to radiosonde data.

**Packet Processing**: For each incoming packet:
    - Checks if the sonde is within the alert boundary
    - Claims a mutex on the sonde (to avoid duplicate processing)
    - If new: sends a Discord webhook with the alert and creates a redis record
    - If existing: updates Discord webhook with prediction and renders a map

**Background**: At start and every 12h, fetch a list of telemetry receivers(stations) from sondehub and store in-memory.

---

## Setup Instructions

1. **Clone the repository**

   ```sh
   git clone https://github.com/mrarm/balloony.git
   cd balloony
   ```

2. **Prepare Environment Variables**

   - Copy `.env.example` to `.env` and fill in the required values, or set them in your environment.

3. **Build Alert Boundaries**

   - The `ALERT_BOUNDS` variable must be a JSON array of `[longitude, latitude]` pairs forming a closed polygon.
   - Example:

     ```json
     [[-95.6399117,39.0869135],[-95.5825768,39.0853146],[-95.5884133,39.0439965],[-95.6443749,39.0485294],[-95.6399117,39.086647]]
     ```

   - You can use [https://www.keene.edu/campus/maps/tool/](https://www.keene.edu/campus/maps/tool/) or similar tools to draw your area and export coordinates.

4. **Obtain a Radar.com API Key**

   - Sign up at [radar.com](https://radar.com/) and create a project to get your API key. The free tier has plenty of calls per month for running this in an area that sees a few radiosondes a day.

5. **Run the Application**

Option 1: Build and Run with Go

```sh
go build -o balloony .
./balloony
```

Option 2: Run with Docker Compose

1. Make sure your `.env` file is present in the project root.
2. Start the service:

    ```sh
    docker-compose up -d --build
    ```

---

## Use in areas outside of the United States

You may need to modify the location parsing code(`GetLocationFromRadarResponse`) inside of `utils.go` as it's currently designed to parse location formats in the form of City, State(two letter code).

If your area does not use this format for it's locations, make sure you update this in the code. Otherwise, you may see missing information in the embed.

## Bundled Launch Site JSON

Usually, the launch site locations on SondeHub don't change all that often. To avoid piling up more API calls, we rely on a local list of launch sites that was pulled from the SondeHub API a while ago. For the author's area, this doesn't have any issues. If you find that your launch area is missing, send out a pull request or pull the file into your local codebase.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

Balloon, Parachute, and Target Icons © Rossen Georgiev, MIT License, [projecthorus/sondehub-tracker](https://github.com/projecthorus/sondehub-tracker).
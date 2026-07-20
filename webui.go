package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
)

const webuiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Balloony Feature Flags</title>
<style>
  body { font-family: sans-serif; max-width: 480px; margin: 40px auto; padding: 0 16px; }
  label { display: block; margin: 16px 0 2px; }
  input[type=text], select { width: 100%; padding: 4px; box-sizing: border-box; }
  button { margin-top: 16px; padding: 4px 16px; }
</style>
</head>
<body>
<h1>Feature Flags</h1>
<p>Changes will not persist after a restart.</p>
<form method="POST" action="/flags/save">
  <h2>Thunderforest Map Tiles</h2>
  <label><input type="checkbox" name="tf_enabled" value="1" {{if .ThunderforestEnabled}}checked{{end}}> Enable Thunderforest tiles</label>
  <label for="tf_style">Style</label>
  <select id="tf_style" name="tf_style">
    {{range .Styles}}
    <option value="{{.}}" {{if eq . $.ThunderforestStyle}}selected{{end}}>{{.}}</option>
    {{end}}
  </select>
  <label for="tf_apikey">API Key</label>
  <input type="text" id="tf_apikey" name="tf_apikey" value="{{.ThunderforestAPIKey}}">
  <br>
  <button type="submit">Save</button>
</form>
{{if .Saved}}<p>Saved.</p>{{end}}
</body>
</html>
`

var thunderforestStyles = []string{
	"cycle",
	"transport",
	"transport-dark",
	"landscape",
	"outdoors",
	"spinal-map",
	"pioneer",
	"mobile-atlas",
	"neighbourhood",
	"atlas",
}

var webuiTmpl = template.Must(template.New("flags").Parse(webuiHTML))

func startWebUI() {
	port := os.Getenv("WEBUI_PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/flags", http.StatusFound)
	})
	http.HandleFunc("/flags", handleFlagsPage)
	http.HandleFunc("/flags/save", handleFlagsSave)

	go func() {
		fmt.Printf("Web UI listening on http://0.0.0.0:%s/flags\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			fmt.Println("Web UI error:", err)
		}
	}()
}

func handleFlagsPage(w http.ResponseWriter, r *http.Request) {
	featureFlagsMu.RLock()
	data := struct {
		ThunderforestEnabled bool
		ThunderforestStyle   string
		ThunderforestAPIKey  string
		Styles               []string
		Saved                bool
	}{
		ThunderforestEnabled: featureFlags.ThunderforestEnabled,
		ThunderforestStyle:   featureFlags.ThunderforestStyle,
		ThunderforestAPIKey:  featureFlags.ThunderforestAPIKey,
		Styles:               thunderforestStyles,
		Saved:                r.URL.Query().Get("saved") == "1",
	}
	featureFlagsMu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := webuiTmpl.Execute(w, data); err != nil {
		fmt.Println("Web UI render error:", err)
	}
}

func handleFlagsSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/flags", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	tfEnabled := r.FormValue("tf_enabled") == "1"
	tfStyle := r.FormValue("tf_style")
	tfAPIKey := r.FormValue("tf_apikey")

	// Validate style is a known value to prevent unexpected inputs
	validStyle := false
	for _, s := range thunderforestStyles {
		if s == tfStyle {
			validStyle = true
			break
		}
	}
	if !validStyle {
		tfStyle = "cycle"
	}

	featureFlagsMu.Lock()
	featureFlags.ThunderforestEnabled = tfEnabled
	featureFlags.ThunderforestStyle = tfStyle
	featureFlags.ThunderforestAPIKey = tfAPIKey
	featureFlagsMu.Unlock()

	fmt.Printf("Feature flags updated: Thunderforest=%v style=%s\n", tfEnabled, tfStyle)

	http.Redirect(w, r, "/flags?saved=1", http.StatusSeeOther)
}

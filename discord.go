package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"
)

type DiscordField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type EmbedImage struct {
	URL    string `json:"url"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

type DiscordEmbed struct {
	Type        string         `json:"type,omitempty"` // usually "rich"
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Url         string         `json:"url,omitempty"`
	Fields      []DiscordField `json:"fields,omitempty"`
	Image       *EmbedImage    `json:"image,omitempty"`
}

type DiscordAttachement struct {
	ID          string `json:"id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Description string `json:"description,omitempty"`
}

type DiscordMessage struct {
	Content     string               `json:"content,omitempty"`
	Embeds      []DiscordEmbed       `json:"embeds,omitempty"`
	Attachments []DiscordAttachement `json:"attachments,omitempty"`
}

type DiscordWebhookResponse struct {
	Type         int    `json:"type"`
	Content      string `json:"content"`
	Mentions     []any  `json:"mentions"`
	MentionRoles []any  `json:"mention_roles"`
	Attachments  []any  `json:"attachments"`
	Embeds       []struct {
		Type   string `json:"type"`
		URL    string `json:"url"`
		Title  string `json:"title"`
		Color  int    `json:"color"`
		Fields []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Inline bool   `json:"inline"`
		} `json:"fields"`
		Image struct {
			URL                string `json:"url"`
			ProxyURL           string `json:"proxy_url"`
			Width              int    `json:"width"`
			Height             int    `json:"height"`
			ContentType        string `json:"content_type"`
			Placeholder        string `json:"placeholder"`
			PlaceholderVersion int    `json:"placeholder_version"`
			Flags              int    `json:"flags"`
		} `json:"image"`
	} `json:"embeds"`
	Timestamp       time.Time `json:"timestamp"`
	EditedTimestamp any       `json:"edited_timestamp"`
	Flags           int       `json:"flags"`
	Components      []any     `json:"components"`
	ID              string    `json:"id"`
	ChannelID       string    `json:"channel_id"`
	Author          struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		Avatar        string `json:"avatar"`
		Discriminator string `json:"discriminator"`
		PublicFlags   int    `json:"public_flags"`
		Flags         int    `json:"flags"`
		Bot           bool   `json:"bot"`
		GlobalName    any    `json:"global_name"`
		Clan          any    `json:"clan"`
		PrimaryGuild  any    `json:"primary_guild"`
	} `json:"author"`
	Pinned          bool   `json:"pinned"`
	MentionEveryone bool   `json:"mention_everyone"`
	Tts             bool   `json:"tts"`
	WebhookID       string `json:"webhook_id"`
}

// SendDiscordWebhook sends a DiscordMessage to the given webhook URL. If edit is true, uses PATCH instead of POST.
// It always appends wait=true and returns the DiscordWebhookResponse.
func SendDiscordWebhook(msg DiscordMessage, webhookURL string, edit bool) (DiscordWebhookResponse, error) {
	var respObj DiscordWebhookResponse
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return respObj, fmt.Errorf("failed to marshal DiscordMessage: %w", err)
	}
	// Ensure ?wait=true is present
	url := webhookURL
	if len(url) > 0 && (url[len(url)-1] == '?' || url[len(url)-1] == '&') {
		url += "wait=true"
	} else if len(url) > 0 && (contains(url, "?")) {
		url += "&wait=true"
	} else {
		url += "?wait=true"
	}
	method := "POST"
	if edit {
		method = "PATCH"
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return respObj, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respObj, fmt.Errorf("discord webhook returned status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return respObj, fmt.Errorf("failed to decode DiscordWebhookResponse: %w", err)
	}
	return respObj, nil
}

func SendUpdatedWebhookWithImage(webhookURL string, embed *DiscordEmbed, imageBuf *bytes.Buffer) (DiscordWebhookResponse, error) {
	// Implant the image into the embed
	imageName := fmt.Sprintf("map_%d.png", time.Now().Unix())
	embedImg := EmbedImage{
		URL: fmt.Sprintf("attachment://%s", imageName),
	}

	embed.Image = &embedImg

	// We make a new attachment set to clear any previous attachments
	attachments := []DiscordAttachement{
		{
			ID:          "0",
			Filename:    imageName,
			Description: "Map image",
		},
	}

	// Wrap up the JSON payload
	payload := DiscordMessage{
		Embeds:      []DiscordEmbed{*embed},
		Attachments: attachments,
	}

	// Create a multipart request
	var respObj DiscordWebhookResponse

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add payload_json part
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return respObj, fmt.Errorf("failed to marshal payload: %w", err)
	}
	if fw, err := w.CreateFormField("payload_json"); err != nil {
		return respObj, fmt.Errorf("failed to create payload_json field: %w", err)
	} else {
		if _, err := fw.Write(payloadBytes); err != nil {
			return respObj, fmt.Errorf("failed to write payload_json: %w", err)
		}
	}

	// Add files[0] part
	if fw, err := w.CreateFormFile("files[0]", imageName); err != nil {
		return respObj, fmt.Errorf("failed to create files[0] field: %w", err)
	} else {
		if _, err := fw.Write(imageBuf.Bytes()); err != nil {
			return respObj, fmt.Errorf("failed to write image data: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return respObj, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := webhookURL
	if len(url) > 0 && (url[len(url)-1] == '?' || url[len(url)-1] == '&') {
		url += "wait=true"
	} else if len(url) > 0 && (contains(url, "?")) {
		url += "&wait=true"
	} else {
		url += "?wait=true"
	}

	req, err := http.NewRequest("PATCH", url, &b)
	if err != nil {
		return respObj, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return respObj, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respObj, fmt.Errorf("discord webhook returned status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return respObj, fmt.Errorf("failed to decode DiscordWebhookResponse: %w", err)
	}
	return respObj, nil
}

// contains returns true if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))
}

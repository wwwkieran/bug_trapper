package identifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
)

type OrganismResult struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	IllustrationURL string `json:"illustration_url,omitempty"`
}

type Identifier interface {
	Identify(ctx context.Context, img image.Image) (*OrganismResult, error)
}

// OpenAIIdentifier calls the OpenAI API for organism identification and illustration.
type OpenAIIdentifier struct {
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string // defaults to https://api.openai.com
}

func (o *OpenAIIdentifier) baseURL() string {
	if o.BaseURL != "" {
		return o.BaseURL
	}
	return "https://api.openai.com/v1"
}

func (o *OpenAIIdentifier) httpClient() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return http.DefaultClient
}

// Identify sends the image to GPT-4o for identification, then generates an illustration.
func (o *OpenAIIdentifier) Identify(ctx context.Context, img image.Image) (*OrganismResult, error) {
	// Encode image to JPEG then base64
	b64, err := imageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("encoding image: %w", err)
	}

	// Step 1: Identify the organism
	result, err := o.identifyOrganism(ctx, b64)
	if err != nil {
		return nil, fmt.Errorf("identifying organism: %w", err)
	}

	// Step 2: Generate illustration
	illustrationURL, err := o.generateIllustration(ctx, result.Name)
	if err != nil {
		return nil, fmt.Errorf("generating illustration: %w", err)
	}
	result.IllustrationURL = illustrationURL

	return result, nil
}

func (o *OpenAIIdentifier) identifyOrganism(ctx context.Context, imageBase64 string) (*OrganismResult, error) {
	body := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": `Identify the organism in this photo. Give a short description of the organism for kids.

Respond in JSON format with exactly these fields:
{"name": "Common Name", "description": "A fun, kid-friendly description in 1-2 sentences."}

Only respond with the JSON, no other text.`,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/jpeg;base64," + imageBase64,
						},
					},
				},
			},
		},
		"max_tokens": 300,
	}

	respBody, err := o.doRequest(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}

	return parseIdentifyResponse(respBody)
}

func (o *OpenAIIdentifier) generateIllustration(ctx context.Context, organismName string) (string, error) {
	body := map[string]interface{}{
		"model":  "dall-e-3",
		"prompt": fmt.Sprintf("A simple black and white illustration of a %s. Bold outlines, high contrast, minimal detail, centered on a white background. Woodcut print style.", organismName),
		"n":      1,
		"size":   "1024x1024",
	}

	respBody, err := o.doRequest(ctx, "/images/generations", body)
	if err != nil {
		return "", err
	}

	return parseImageResponse(respBody)
}

func (o *OpenAIIdentifier) doRequest(ctx context.Context, path string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL()+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func parseIdentifyResponse(body []byte) (*OrganismResult, error) {
	// Parse the top-level response to inspect structure
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w\nraw body: %s", err, string(body))
	}

	// Try standard OpenAI chat completions format
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response structure: %w\nraw body: %s", err, string(body))
	}

	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	if content == "" {
		// Dump the raw response so we can debug what the API actually returned
		return nil, fmt.Errorf("empty content in API response.\nraw body: %s", string(body))
	}

	// Strip potential markdown code fences
	content = stripCodeFences(content)

	var result OrganismResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parsing organism JSON: %w\ncontent was: %s", err, content)
	}

	if result.Name == "" {
		return nil, fmt.Errorf("empty organism name in response.\ncontent was: %s", content)
	}

	return &result, nil
}

func parseImageResponse(body []byte) (string, error) {
	var resp struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parsing image response: %w", err)
	}

	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no images in response")
	}

	return resp.Data[0].URL, nil
}

func stripCodeFences(s string) string {
	// Remove ```json ... ``` wrapping if present
	s = bytes.NewBufferString(s).String()
	if len(s) > 7 && s[:7] == "```json" {
		s = s[7:]
	} else if len(s) > 3 && s[:3] == "```" {
		s = s[3:]
	}
	if len(s) > 3 && s[len(s)-3:] == "```" {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

func imageToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

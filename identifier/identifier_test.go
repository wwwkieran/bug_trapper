package identifier

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}
	return img
}

func TestIdentifyOrganism(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			// Verify request structure
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			json.Unmarshal(body, &req)

			if req["model"] != "gpt-4o" {
				t.Errorf("expected model gpt-4o, got %v", req["model"])
			}

			// Check authorization header
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-key" {
				t.Errorf("expected Bearer test-key, got %s", auth)
			}

			// Return mock response
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": `{"name": "Monarch Butterfly", "description": "A beautiful orange and black butterfly that flies thousands of miles!"}`,
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/images/generations" {
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			json.Unmarshal(body, &req)

			prompt, _ := req["prompt"].(string)
			if !strings.Contains(prompt, "Monarch Butterfly") {
				t.Errorf("illustration prompt should contain organism name, got: %s", prompt)
			}

			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"url": "https://example.com/butterfly.png"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	ident := &OpenAIIdentifier{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	result, err := ident.Identify(context.Background(), testImage())
	if err != nil {
		t.Fatalf("Identify failed: %v", err)
	}

	if result.Name != "Monarch Butterfly" {
		t.Errorf("expected name 'Monarch Butterfly', got %q", result.Name)
	}
	if result.Description == "" {
		t.Error("expected non-empty description")
	}
	if result.IllustrationURL != "https://example.com/butterfly.png" {
		t.Errorf("expected illustration URL, got %q", result.IllustrationURL)
	}
}

func TestIdentifyWithCodeFences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": "```json\n{\"name\": \"Ladybug\", \"description\": \"A tiny red beetle with black spots!\"}\n```",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/images/generations" {
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"url": "https://example.com/ladybug.png"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	ident := &OpenAIIdentifier{APIKey: "test-key", BaseURL: server.URL}
	result, err := ident.Identify(context.Background(), testImage())
	if err != nil {
		t.Fatalf("Identify failed: %v", err)
	}
	if result.Name != "Ladybug" {
		t.Errorf("expected 'Ladybug', got %q", result.Name)
	}
}

func TestIdentifyAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	ident := &OpenAIIdentifier{APIKey: "test-key", BaseURL: server.URL}
	_, err := ident.Identify(context.Background(), testImage())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestIdentifyEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ident := &OpenAIIdentifier{APIKey: "test-key", BaseURL: server.URL}
	_, err := ident.Identify(context.Background(), testImage())
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestParseIdentifyResponse(t *testing.T) {
	body := `{"choices":[{"message":{"content":"{\"name\":\"Ant\",\"description\":\"Tiny but mighty!\"}"}}]}`
	result, err := parseIdentifyResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseIdentifyResponse failed: %v", err)
	}
	if result.Name != "Ant" {
		t.Errorf("expected 'Ant', got %q", result.Name)
	}
}

func TestParseImageResponse(t *testing.T) {
	body := `{"data":[{"url":"https://example.com/img.png"}]}`
	url, err := parseImageResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseImageResponse failed: %v", err)
	}
	if url != "https://example.com/img.png" {
		t.Errorf("expected URL, got %q", url)
	}
}

func TestParseImageResponseEmpty(t *testing.T) {
	body := `{"data":[]}`
	_, err := parseImageResponse([]byte(body))
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestStripCodeFences(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{`{"name":"X"}`, `{"name":"X"}`},
		{"```json\n{\"name\":\"X\"}\n```", `{"name":"X"}`},
		{"```\n{\"name\":\"X\"}\n```", `{"name":"X"}`},
	}
	for _, tc := range cases {
		got := stripCodeFences(tc.input)
		if got != tc.want {
			t.Errorf("stripCodeFences(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestImageToBase64(t *testing.T) {
	img := testImage()
	b64, err := imageToBase64(img)
	if err != nil {
		t.Fatalf("imageToBase64 failed: %v", err)
	}
	if len(b64) == 0 {
		t.Error("expected non-empty base64 string")
	}
}

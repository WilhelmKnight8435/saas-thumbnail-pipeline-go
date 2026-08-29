package thumbnail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProcessRequestBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/image/process" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "acme-upload-card" {
			t.Fatal("authentication or idempotency header missing")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		for field, want := range map[string]string{"ops": `[{"enlarge":false,"fit":"cover","height":360,"type":"resize","width":640}]`, "format": "webp", "store": "true"} {
			if got := r.FormValue(field); got != want {
				t.Errorf("field %s = %q, want %q", field, got, want)
			}
		}
		for field := range r.MultipartForm.Value {
			if field != "ops" && field != "format" && field != "store" {
				t.Errorf("unsupported field %s", field)
			}
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		got, _ := io.ReadAll(file)
		if string(got) != "pixels" {
			t.Errorf("image = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"id": "img_123", "url": "https://cdn.example/thumb.webp"}, "error": nil, "metadata": map[string]string{"vendor": "auto"}})
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 2}
	got, err := client.Process(context.Background(), []byte("pixels"), "source.png", Variant{Width: 640, Height: 360, Fit: "cover", Format: "webp"}, "acme-upload-card")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "img_123" {
		t.Fatalf("result ID = %q", got.ID)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"example.com/saas-thumbnail-pipeline/thumbnail"
)

type service struct {
	mu      sync.RWMutex
	tenants map[string]thumbnail.Tenant
	client  *thumbnail.Client
}

func main() {
	client := &thumbnail.Client{APIKey: os.Getenv("INFRAI_API_KEY"), MaxRetries: 3}
	s := &service{client: client, tenants: make(map[string]thumbnail.Tenant)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tenants/{id}/onboard", s.onboard)
	mux.HandleFunc("POST /admin/tenants/{id}/state/{state}", s.setState)
	mux.HandleFunc("POST /tenants/{id}/thumbnails", s.generate)
	log.Printf("thumbnail service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *service) onboard(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ThumbnailProfile string `json:"thumbnail_profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || (input.ThumbnailProfile != "catalog" && input.ThumbnailProfile != "avatar") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "thumbnail_profile must be catalog or avatar"})
		return
	}
	tenant := thumbnail.Tenant{ID: r.PathValue("id"), State: thumbnail.AccountActive, OnboardingDone: true, ThumbnailProfile: input.ThumbnailProfile}
	s.mu.Lock()
	s.tenants[tenant.ID] = tenant
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, tenant)
}

func (s *service) setState(w http.ResponseWriter, r *http.Request) {
	state := thumbnail.AccountState(r.PathValue("state"))
	if state != thumbnail.AccountActive && state != thumbnail.AccountSuspended {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "state must be active or suspended"})
		return
	}
	s.mu.Lock()
	tenant, ok := s.tenants[r.PathValue("id")]
	if ok {
		tenant.State = state
		s.tenants[tenant.ID] = tenant
	}
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tenant not found"})
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (s *service) generate(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	tenant, ok := s.tenants[r.PathValue("id")]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tenant not found"})
		return
	}
	variants, err := thumbnail.VariantsFor(tenant)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(key) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key header is required"})
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart upload"})
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}
	defer file.Close()
	image, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read image"})
		return
	}

	results := make(map[string]thumbnail.ProcessResult, len(variants))
	for _, variant := range variants {
		result, err := s.client.Process(r.Context(), image, header.Filename, variant, fmt.Sprintf("%s-%s", key, variant.Name))
		if err != nil {
			var apiErr *thumbnail.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "thumbnail processing did not complete"})
			return
		}
		results[variant.Name] = result
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tenant_id": tenant.ID, "thumbnails": results})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

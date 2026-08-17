package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/infrai-examples/fieldservice-photo-handoff/internal/fieldassets"
)

func main() {
	client := fieldassets.NewClient(os.Getenv("INFRAI_API_KEY"))
	service := fieldassets.NewService(client, envOr("ASSET_BUCKET", "field-service-assets"))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /work-orders/uploads", func(w http.ResponseWriter, r *http.Request) {
		var input fieldassets.UploadRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		ticket, err := service.IssueUpload(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, ticket)
	})
	mux.HandleFunc("POST /work-orders/{workOrderID}/photos/{photoID}/confirm", func(w http.ResponseWriter, r *http.Request) {
		technicianID := r.URL.Query().Get("technician_id")
		result, err := service.ConfirmUpload(r.Context(), r.PathValue("workOrderID"), technicianID, r.PathValue("photoID"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	address := envOr("LISTEN_ADDR", ":8080")
	log.Printf("field-assets listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *fieldassets.APIError
	var validationErr *fieldassets.ValidationError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	} else if errors.As(err, &validationErr) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

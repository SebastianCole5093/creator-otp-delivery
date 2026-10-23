package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/example/creator-otp-delivery/internal/creator"
	"github.com/example/creator-otp-delivery/internal/infrai"
)

type server struct {
	login *creator.LoginService
}

type startRequest struct {
	Phone     string `json:"phone"`
	CreatorID string `json:"creator_id"`
	AssetID   string `json:"asset_id"`
}

type completeRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	app := &server{login: creator.NewLoginService(infrai.NewSMSClient(key))}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code", app.start)
	mux.HandleFunc("POST /login/verify", app.complete)

	address := ":8080"
	log.Printf("creator login listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func (s *server) start(w http.ResponseWriter, r *http.Request) {
	var input startRequest
	if err := decode(r, &input); err != nil || input.Phone == "" || input.CreatorID == "" || input.AssetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone, creator_id, and asset_id are required"})
		return
	}
	id, err := s.login.Start(r.Context(), input.Phone, input.CreatorID, input.AssetID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"challenge_id": id, "expires_in_seconds": 600})
}

func (s *server) complete(w http.ResponseWriter, r *http.Request) {
	var input completeRequest
	if err := decode(r, &input); err != nil || input.ChallengeID == "" || input.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "challenge_id and code are required"})
		return
	}
	result, err := s.login.Complete(r.Context(), input.ChallengeID, input.Code)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decode(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	}
	if errors.Is(err, creator.ErrChallengeMissing) || errors.Is(err, creator.ErrChallengeUsed) {
		status = http.StatusConflict
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

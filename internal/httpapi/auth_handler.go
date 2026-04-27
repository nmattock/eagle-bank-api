package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"eagle-bank-api/internal/auth"
	"eagle-bank-api/internal/users"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	repo   users.Repository
	tokens *auth.TokenService
}

func NewAuthHandler(repo users.Repository, tokens *auth.TokenService) *AuthHandler {
	return &AuthHandler{repo: repo, tokens: tokens}
}

type authenticateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authenticationResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/auth/token" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req authenticateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "invalid JSON"})
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "email is required"})
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "password is required"})
		return
	}

	creds, err := h.repo.GetAuthCredentialsByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid credentials"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(creds.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "invalid credentials"})
		return
	}

	token, err := h.tokens.Issue(creds.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, authenticationResponse{Token: token})
}

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"eagle-bank-api/internal/auth"
	"eagle-bank-api/internal/users"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	repo   users.Repository
	tokens *auth.TokenService
}

func NewUserHandler(repo users.Repository, tokens ...*auth.TokenService) *UserHandler {
	var tokenService *auth.TokenService
	if len(tokens) > 0 {
		tokenService = tokens[0]
	}
	return &UserHandler{repo: repo, tokens: tokenService}
}

type createUserAddressRequest struct {
	Line1    string `json:"line1"`
	Line2    string `json:"line2"`
	Line3    string `json:"line3"`
	Town     string `json:"town"`
	County   string `json:"county"`
	Postcode string `json:"postcode"`
}

type createUserRequest struct {
	Name        string                   `json:"name"`
	Address     createUserAddressRequest `json:"address"`
	PhoneNumber string                   `json:"phoneNumber"`
	Email       string                   `json:"email"`
	Password    string                   `json:"password"`
}

type errorResponse struct {
	Message string `json:"message"`
}

type userAddressResponse struct {
	Line1    string  `json:"line1"`
	Line2    *string `json:"line2,omitempty"`
	Line3    *string `json:"line3,omitempty"`
	Town     string  `json:"town"`
	County   string  `json:"county"`
	Postcode string  `json:"postcode"`
}

type userResponse struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Address          userAddressResponse `json:"address"`
	PhoneNumber      string              `json:"phoneNumber"`
	Email            string              `json:"email"`
	CreatedTimestamp string              `json:"createdTimestamp"`
	UpdatedTimestamp string              `json:"updatedTimestamp"`
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/users/") {
		h.handleUserByID(w, r)
		return
	}
	if r.URL.Path != "/v1/users" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "invalid JSON"})
		return
	}

	if err := validateCreateUserRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: err.Error()})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}

	params := users.CreateParams{
		ID:           "usr-" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", ""),
		Name:         req.Name,
		AddressLine1: req.Address.Line1,
		Town:         req.Address.Town,
		County:       req.Address.County,
		Postcode:     req.Address.Postcode,
		PhoneNumber:  req.PhoneNumber,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
	}
	if req.Address.Line2 != "" {
		line2 := req.Address.Line2
		params.AddressLine2 = &line2
	}
	if req.Address.Line3 != "" {
		line3 := req.Address.Line3
		params.AddressLine3 = &line3
	}

	createdUser, err := h.repo.Create(context.Background(), params)
	if err != nil {
		if errors.Is(err, users.ErrAlreadyExists) {
			writeJSON(w, http.StatusConflict, errorResponse{Message: "user already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toUserResponse(createdUser))
}

func (h *UserHandler) handleUserByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/v1/users/")
	if userID == "" || strings.Contains(userID, "/") {
		http.NotFound(w, r)
		return
	}

	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if claims.Subject != userID {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}

	u, err := h.repo.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "user not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

func (h *UserHandler) authenticate(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	if h.tokens == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	token, ok := bearerToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	claims, err := h.tokens.Verify(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	return claims, true
}

func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

func validateCreateUserRequest(req createUserRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(req.Address.Line1) == "" {
		return errors.New("address.line1 is required")
	}
	if strings.TrimSpace(req.Address.Town) == "" {
		return errors.New("address.town is required")
	}
	if strings.TrimSpace(req.Address.County) == "" {
		return errors.New("address.county is required")
	}
	if strings.TrimSpace(req.Address.Postcode) == "" {
		return errors.New("address.postcode is required")
	}
	if strings.TrimSpace(req.PhoneNumber) == "" {
		return errors.New("phoneNumber is required")
	}
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func toUserResponse(u *users.User) userResponse {
	return userResponse{
		ID:   u.ID,
		Name: u.Name,
		Address: userAddressResponse{
			Line1:    u.AddressLine1,
			Line2:    u.AddressLine2,
			Line3:    u.AddressLine3,
			Town:     u.Town,
			County:   u.County,
			Postcode: u.Postcode,
		},
		PhoneNumber:      u.PhoneNumber,
		Email:            u.Email,
		CreatedTimestamp: u.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedTimestamp: u.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

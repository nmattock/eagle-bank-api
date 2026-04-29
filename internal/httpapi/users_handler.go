package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"eagle-bank-api/internal/users"

	"github.com/google/uuid"

	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	repo users.Repository
}

func NewUserHandler(repo users.Repository) *UserHandler {
	return &UserHandler{repo: repo}
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
		slog.ErrorContext(r.Context(), "failed to hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}

	params := users.CreateParams{
		ID:           generateUserID(),
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

	createdUser, err := h.repo.Create(r.Context(), params)
	if err != nil {
		if errors.Is(err, users.ErrAlreadyExists) {
			writeJSON(w, http.StatusConflict, errorResponse{Message: "user already exists"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to create user", "error", err, "email", req.Email)
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

	claims := ClaimsFromContext(r.Context())

	u, err := h.repo.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "user not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to fetch user", "error", err, "user_id", userID, "requester_user_id", claims.Subject)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if claims.Subject != userID {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
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

func generateUserID() string {
	// OpenAPI/DB require ^usr-[A-Za-z0-9]+$.
	// UUID entropy is desirable, but canonical UUIDs include hyphens; strip them
	// so IDs remain spec-compliant while being hard to guess.
	return "usr-" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

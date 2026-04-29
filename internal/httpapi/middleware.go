package httpapi

import (
	"context"
	"net/http"
	"strings"

	"eagle-bank-api/internal/auth"
)

type claimsContextKey struct{}

func AuthMiddleware(tokens *auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
				return
			}
			claims, err := tokens.Verify(token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsContextKey{}).(*auth.Claims)
	return claims
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return token, token != ""
}

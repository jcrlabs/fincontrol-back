package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jonathanCaamano/fincontrol-back/internal/app"
)

type authService interface {
	Register(ctx context.Context, input app.RegisterInput) (app.TokenPair, error)
	Login(ctx context.Context, input app.LoginInput) (app.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (app.TokenPair, error)
}

// AuthHandler handles /api/v1/auth/* routes.
type AuthHandler struct {
	svc             authService
	refreshTokenTTL time.Duration
}

func NewAuthHandler(svc authService, refreshTokenTTL time.Duration) *AuthHandler {
	return &AuthHandler{svc: svc, refreshTokenTTL: refreshTokenTTL}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password and name are required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	pair, err := h.svc.Register(r.Context(), app.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusCreated, map[string]string{"access_token": pair.AccessToken})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	pair, err := h.svc.Login(r.Context(), app.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"access_token": pair.AccessToken})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "refresh token missing"})
		return
	}

	pair, err := h.svc.RefreshToken(r.Context(), cookie.Value)
	if err != nil {
		writeError(w, err)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"access_token": pair.AccessToken})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTokenTTL.Seconds()),
	})
}

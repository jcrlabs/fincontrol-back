package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
)

type categoryService interface {
	Create(ctx context.Context, input app.CreateCategoryInput) (domain.Category, error)
	List(ctx context.Context, userID uuid.UUID) ([]domain.Category, error)
	Update(ctx context.Context, input app.UpdateCategoryInput) (domain.Category, error)
}

// CategoryHandler handles /api/v1/categories routes.
type CategoryHandler struct {
	svc categoryService
}

func NewCategoryHandler(svc categoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

type categoryResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parent_id,omitempty"`
	Icon      string  `json:"icon,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func toCategoryResponse(c domain.Category) categoryResponse {
	resp := categoryResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Icon:      c.Icon,
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if c.ParentID != nil {
		s := c.ParentID.String()
		resp.ParentID = &s
	}
	return resp
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	cats, err := h.svc.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]categoryResponse, len(cats))
	for i, c := range cats {
		resp[i] = toCategoryResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createCategoryRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	Icon     string `json:"icon,omitempty"`
}

func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req createCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	input := app.CreateCategoryInput{
		UserID: userID,
		Name:   req.Name,
		Icon:   req.Icon,
	}
	if req.ParentID != "" {
		pid, err := uuid.Parse(req.ParentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parent_id"})
			return
		}
		input.ParentID = &pid
	}
	cat, err := h.svc.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCategoryResponse(cat))
}

type updateCategoryRequest struct {
	Name     string  `json:"name,omitempty"`
	ParentID *string `json:"parent_id"`
	Icon     string  `json:"icon,omitempty"`
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category id"})
		return
	}
	var req updateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	input := app.UpdateCategoryInput{
		ID:     id,
		UserID: userID,
		Name:   req.Name,
		Icon:   req.Icon,
	}
	if req.ParentID != nil && *req.ParentID != "" {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parent_id"})
			return
		}
		input.ParentID = &pid
	}
	cat, err := h.svc.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryResponse(cat))
}

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
)

// CategoryService handles hierarchical category management.
type CategoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

type CreateCategoryInput struct {
	UserID   uuid.UUID
	Name     string
	ParentID *uuid.UUID
	Icon     string
}

func (s *CategoryService) Create(ctx context.Context, input CreateCategoryInput) (domain.Category, error) {
	if input.Name == "" {
		return domain.Category{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	cat := domain.Category{
		ID:        uuid.New(),
		UserID:    input.UserID,
		Name:      input.Name,
		ParentID:  input.ParentID,
		Icon:      input.Icon,
		CreatedAt: time.Now().UTC(),
	}
	return s.repo.Create(ctx, cat)
}

func (s *CategoryService) List(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	return s.repo.ListByUser(ctx, userID)
}

type UpdateCategoryInput struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Name     string
	ParentID *uuid.UUID
	Icon     string
}

func (s *CategoryService) Update(ctx context.Context, input UpdateCategoryInput) (domain.Category, error) {
	cats, err := s.repo.ListByUser(ctx, input.UserID)
	if err != nil {
		return domain.Category{}, err
	}
	var cat *domain.Category
	for i := range cats {
		if cats[i].ID == input.ID {
			cat = &cats[i]
			break
		}
	}
	if cat == nil {
		return domain.Category{}, domain.ErrNotFound
	}
	if input.Name != "" {
		cat.Name = input.Name
	}
	if input.Icon != "" {
		cat.Icon = input.Icon
	}
	cat.ParentID = input.ParentID
	return s.repo.Update(ctx, *cat)
}

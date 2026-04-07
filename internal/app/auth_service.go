package app

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// TokenPair holds tokens and the authenticated user info.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	Email        string
	Name         string
}

// AuthService handles registration, login, and token refresh.
type AuthService struct {
	repo            AuthRepository
	jwtPrivateKey   interface{} // *rsa.PrivateKey
	jwtPublicKey    interface{} // *rsa.PublicKey
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewAuthService(repo AuthRepository, privateKey, publicKey interface{}, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		repo:            repo,
		jwtPrivateKey:   privateKey,
		jwtPublicKey:    publicKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (TokenPair, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	user := domain.User{
		ID:           uuid.New(),
		Email:        input.Email,
		Name:         input.Name,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	created, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		return TokenPair{}, err
	}

	pair, err := s.issueTokens(created.ID)
	if err != nil {
		return TokenPair{}, err
	}
	pair.UserID = created.ID.String()
	pair.Email = created.Email
	pair.Name = created.Name
	return pair, nil
}

type LoginInput struct {
	Email    string
	Password string
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}

	pair, err := s.issueTokens(user.ID)
	if err != nil {
		return TokenPair{}, err
	}
	pair.UserID = user.ID.String()
	pair.Email = user.Email
	pair.Name = user.Name
	return pair, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}

	if claims["type"] != "refresh" {
		return TokenPair{}, domain.ErrUnauthorized
	}

	subStr, ok := claims["sub"].(string)
	if !ok {
		return TokenPair{}, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(subStr)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}

	// Verify user still exists
	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}

	return s.issueTokens(userID)
}

func (s *AuthService) ValidateAccessToken(tokenString string) (uuid.UUID, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return uuid.Nil, domain.ErrUnauthorized
	}
	if claims["type"] != "access" {
		return uuid.Nil, domain.ErrUnauthorized
	}
	subStr, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return uuid.Parse(subStr)
}

func (s *AuthService) issueTokens(userID uuid.UUID) (TokenPair, error) {
	now := time.Now().UTC()

	accessToken, err := s.signToken(jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"iat":  now.Unix(),
		"exp":  now.Add(s.accessTokenTTL).Unix(),
	})
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := s.signToken(jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(s.refreshTokenTTL).Unix(),
	})
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign refresh token: %w", err)
	}

	return TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *AuthService) signToken(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.jwtPrivateKey)
}

func (s *AuthService) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtPublicKey, nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

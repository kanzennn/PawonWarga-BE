package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
	"PawonWarga-BE/pkg/jwtutil"
	"PawonWarga-BE/pkg/storage"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrEmailTaken           = errors.New("email already registered")
	ErrInvalidCreds         = errors.New("invalid email or password")
	ErrUserNotFound         = errors.New("user not found")
	ErrStorageNotConfigured = errors.New("file storage is not configured")
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type UpdateProfileInput struct {
	Name string
}

type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*model.User, error)
	Login(ctx context.Context, email, password string) (token string, user *model.User, err error)
	GetProfile(ctx context.Context, userID uint) (*model.User, error)
	UpdateProfile(ctx context.Context, userID uint, input UpdateProfileInput) (*model.User, error)
	UploadProfilePicture(ctx context.Context, userID uint, filename string, body io.Reader, size int64, contentType string) (*model.User, error)
}

type authService struct {
	userRepo  repository.UserRepository
	storage   storage.Storage
	jwtSecret string
	jwtExpiry int
}

func NewAuthService(userRepo repository.UserRepository, stor storage.Storage, jwtSecret string, jwtExpiry int) AuthService {
	return &authService{
		userRepo:  userRepo,
		storage:   stor,
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
	}
}

func (s *authService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	existing, err := s.userRepo.FindByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{Name: input.Name, Email: input.Email, Password: string(hashed)}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, ErrInvalidCreds
		}
		return "", nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, ErrInvalidCreds
	}

	token, err := jwtutil.GenerateToken(user.ID, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

func (s *authService) GetProfile(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *authService) UpdateProfile(ctx context.Context, userID uint, input UpdateProfileInput) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user.Name = input.Name
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) UploadProfilePicture(ctx context.Context, userID uint, filename string, body io.Reader, size int64, contentType string) (*model.User, error) {
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Delete previous picture before uploading the new one (best effort)
	if user.ProfilePictureKey != nil {
		_ = s.storage.Delete(ctx, *user.ProfilePictureKey)
	}

	key, err := generateKey(userID, filename)
	if err != nil {
		return nil, err
	}

	url, err := s.storage.Upload(ctx, storage.UploadInput{
		Key:         key,
		Body:        body,
		Size:        size,
		ContentType: contentType,
	})
	if err != nil {
		return nil, err
	}

	user.ProfilePicture = &url
	user.ProfilePictureKey = &key
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// generateKey produces a unique, collision-resistant S3 key for a profile picture.
func generateKey(userID uint, filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	return fmt.Sprintf("profiles/%d/%x%s", userID, b, ext), nil
}

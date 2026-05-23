package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"PawonWarga-BE/internal/config"
	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/router"
	"PawonWarga-BE/pkg/cache"
	"PawonWarga-BE/pkg/database"
	"PawonWarga-BE/pkg/storage"
	"gorm.io/gorm"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	db         *gorm.DB
	cache      *cache.Cache
}

func New(cfg *config.Config) (*Server, error) {
	db, err := database.NewPostgres(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	if err := db.AutoMigrate(&model.User{}); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}

	var cacheClient *cache.Cache
	if cfg.Cache.Enabled {
		cacheClient, err = cache.NewRedis(&cfg.Cache)
		if err != nil {
			return nil, fmt.Errorf("cache: %w", err)
		}
	}

	// Storage is optional — skipped when STORAGE_BUCKET is not set
	var stor storage.Storage
	if cfg.Storage.Bucket != "" {
		stor, err = storage.NewS3(&cfg.Storage)
		if err != nil {
			return nil, fmt.Errorf("storage: %w", err)
		}
	}

	r := router.New(cfg, db, cacheClient, stor)
	engine := r.Setup()

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
			Handler:      engine,
			ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		},
		cfg:   cfg,
		db:    db,
		cache: cacheClient,
	}, nil
}

func (s *Server) Run() error {
	fmt.Printf("server listening on %s\n", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(s.cfg.Server.ShutdownTimeout)*time.Second,
	)
	defer cancel()

	if s.db != nil {
		if sqlDB, err := s.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}

	return s.httpServer.Shutdown(ctx)
}

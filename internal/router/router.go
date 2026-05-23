package router

import (
	"PawonWarga-BE/internal/config"
	"PawonWarga-BE/internal/handler"
	"PawonWarga-BE/internal/middleware"
	"PawonWarga-BE/internal/repository"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/cache"
	"PawonWarga-BE/pkg/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Router struct {
	engine      *gin.Engine
	cfg         *config.Config
	db          *gorm.DB
	cache       *cache.Cache
	authHandler *handler.AuthHandler
}

func New(cfg *config.Config, db *gorm.DB, cacheClient *cache.Cache, stor storage.Storage) *Router {
	gin.SetMode(cfg.Server.Mode)

	engine := gin.New()
	engine.Use(middleware.Logger())
	engine.Use(gin.Recovery())
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, stor, cfg.JWT.Secret, cfg.JWT.ExpiryHours)

	return &Router{
		engine:      engine,
		cfg:         cfg,
		db:          db,
		cache:       cacheClient,
		authHandler: handler.NewAuthHandler(authSvc),
	}
}

func (r *Router) Setup() *gin.Engine {
	// Swagger UI
	r.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	r.engine.GET("/health", handler.NewHealthHandler().Health)

	// Public auth routes — no authentication required
	public := r.engine.Group("/api/v1/auth")
	{
		public.POST("/register", r.authHandler.Register)
		public.POST("/login", r.authHandler.Login)
	}

	// User routes — JWT required
	user := r.engine.Group("/api/v1/auth")
	user.Use(middleware.JWTAuth(r.cfg.JWT.Secret))
	{
		user.GET("/profile", r.authHandler.GetProfile)
		user.PUT("/profile", r.authHandler.UpdateProfile)
		user.POST("/profile/picture", r.authHandler.UploadProfilePicture)
	}

	// Other API routes — Basic Auth required
	v1 := r.engine.Group("/api/v1")
	v1.Use(middleware.BasicAuth(&r.cfg.Auth))
	{
		// Register your feature handlers here. Example:
		//
		// menuRepo    := repository.NewMenuRepository(r.db)
		// menuSvc     := service.NewMenuService(menuRepo, r.cache)
		// menuHandler := handler.NewMenuHandler(menuSvc)
		//
		// menus := v1.Group("/menus")
		// menus.GET("",         menuHandler.List)
		// menus.POST("",        menuHandler.Create)
		// menus.GET("/:id",     menuHandler.GetByID)
		// menus.PUT("/:id",     menuHandler.Update)
		// menus.DELETE("/:id",  menuHandler.Delete)
	}

	return r.engine
}

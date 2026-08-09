package router

import (
	"PawonWarga-BE/internal/config"
	"PawonWarga-BE/internal/handler"
	"PawonWarga-BE/internal/middleware"
	"PawonWarga-BE/internal/repository"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/cache"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Router struct {
	engine           *gin.Engine
	cfg              *config.Config
	db               *gorm.DB
	cache            *cache.Cache
	userRepo         repository.UserRepository
	authHandler      *handler.AuthHandler
	mentionHandler   *handler.MentionHandler
	ingestHandler    *handler.IngestHandler
	dashboardHandler *handler.DashboardHandler
	sentimentHandler *handler.SentimentHandler
	keywordHandler   *handler.KeywordHandler
}

func New(cfg *config.Config, db *gorm.DB, cacheClient *cache.Cache, stor storage.Storage) *Router {
	gin.SetMode(cfg.Server.Mode)

	engine := gin.New()
	engine.Use(middleware.Logger())
	engine.Use(gin.Recovery())
	// Resolves ?lang= or Accept-Language into the request context so
	// pkg/response and handlers can localize messages (id/en).
	engine.Use(i18n.Middleware())
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, stor, cfg.JWT.Secret, cfg.JWT.ExpiryHours)

	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	mentionSvc := service.NewMentionService(postRepo, commentRepo)
	ingestSvc := service.NewIngestService(postRepo, commentRepo)
	dashboardSvc := service.NewDashboardService(postRepo)
	sentimentSvc := service.NewSentimentService(postRepo)
	keywordSvc := service.NewKeywordService(postRepo)

	return &Router{
		engine:           engine,
		cfg:              cfg,
		db:               db,
		cache:            cacheClient,
		userRepo:         userRepo,
		authHandler:      handler.NewAuthHandler(authSvc),
		mentionHandler:   handler.NewMentionHandler(mentionSvc),
		ingestHandler:    handler.NewIngestHandler(ingestSvc),
		dashboardHandler: handler.NewDashboardHandler(dashboardSvc),
		sentimentHandler: handler.NewSentimentHandler(sentimentSvc),
		keywordHandler:   handler.NewKeywordHandler(keywordSvc),
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
	user.Use(middleware.JWTAuth(r.cfg.JWT.Secret, r.userRepo))
	{
		user.GET("/profile", r.authHandler.GetProfile)
		user.PUT("/profile", r.authHandler.UpdateProfile)
		user.POST("/profile/picture", r.authHandler.UploadProfilePicture)
		user.PUT("/password", r.authHandler.ChangePassword)
		user.POST("/logout-all", r.authHandler.LogoutAllDevices)
	}

	// Mention routes — JWT required (analyst-facing dashboard data)
	mentions := r.engine.Group("/api/v1/mentions")
	mentions.Use(middleware.JWTAuth(r.cfg.JWT.Secret, r.userRepo))
	{
		mentions.GET("", r.mentionHandler.List)
		mentions.GET("/:id", r.mentionHandler.GetByID)
		mentions.GET("/:id/comments", r.mentionHandler.ListComments)
	}

	// Dashboard routes — JWT required (analyst-facing overview data)
	dashboard := r.engine.Group("/api/v1/dashboard")
	dashboard.Use(middleware.JWTAuth(r.cfg.JWT.Secret, r.userRepo))
	{
		dashboard.GET("/overview", r.dashboardHandler.GetOverview)
	}

	// Sentiment routes — JWT required (analyst-facing sentiment analysis data)
	sentiment := r.engine.Group("/api/v1/sentiment")
	sentiment.Use(middleware.JWTAuth(r.cfg.JWT.Secret, r.userRepo))
	{
		sentiment.GET("/overview", r.sentimentHandler.GetOverview)
	}

	// Keyword routes — JWT required (analyst-facing keyword analysis data)
	keywords := r.engine.Group("/api/v1/keywords")
	keywords.Use(middleware.JWTAuth(r.cfg.JWT.Secret, r.userRepo))
	{
		keywords.GET("", r.keywordHandler.List)
	}

	// Ingest routes — internal service-to-service, shared-secret API key
	// (see internal/middleware/apikey.go). Used by the Python sentiment-
	// labeling worker to push crawled/labeled posts and comments.
	ingest := r.engine.Group("/api/v1/ingest")
	ingest.Use(middleware.APIKeyAuth(r.cfg.Ingest.APIKey))
	{
		ingest.POST("/posts", r.ingestHandler.IngestPost)
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

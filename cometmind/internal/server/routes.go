package server

import (
	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/gin-gonic/gin"
)

func newEngine(app *App) *gin.Engine {
	r := gin.New()
	r.Use(logging.Gin())
	r.Use(localCORS())
	r.Use(gin.Recovery())
	r.Use(stageRequestBody())
	registerSpecRoutes(r, app)
	registerExcludedRoutes(r, app)
	return r
}

func registerSpecRoutes(r *gin.Engine, app *App) {
	handler := apigen.NewStrictHandlerWithOptions(&strictServer{app: app}, nil, strictHandlerOptions())
	apigen.RegisterHandlersWithOptions(r, handler, apigen.GinServerOptions{
		ErrorHandler: strictParamError,
	})
}

// Routes the strict generator cannot express stay hand-registered: SSE streams
// and byte or attachment downloads.
func registerExcludedRoutes(r *gin.Engine, app *App) {
	r.POST("/api/v1/sessions/:id/messages", func(c *gin.Context) {
		useOriginalBody(c)
		app.handlePostMessage(c)
	})
	r.GET("/api/v1/sessions/:id/events", app.handleSessionEvents)
	r.GET("/api/v1/events", app.handleEvents)
	r.GET("/api/v1/sessions/:id/media/:mediaId", app.handleGetSessionMedia)
	r.GET("/api/v1/media/:id/content", app.handleGetMediaContent)
	r.GET("/api/v1/skills/:name/archive", app.handleExportSkill)
}

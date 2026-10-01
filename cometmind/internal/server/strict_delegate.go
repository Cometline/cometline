package server

import (
	"context"
	"net/http"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/gin-gonic/gin"
)

// strictServer adapts the generated strict interface onto the existing Gin
// handlers. Each method restores the original request body (the generated
// binder consumes a stand-in) and lets the handler write the response.
type strictServer struct {
	app *App
}

func delegateGin(ctx context.Context, handle func(*gin.Context)) {
	c, ok := ctx.(*gin.Context)
	if !ok || c == nil {
		return
	}
	useOriginalBody(c)
	handle(c)
}

func strictHandlerOptions() apigen.StrictGinServerOptions {
	return apigen.StrictGinServerOptions{
		RequestErrorHandlerFunc: func(c *gin.Context, _ error) {
			writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		},
		HandlerErrorFunc: func(c *gin.Context, err error) {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		},
		ResponseErrorHandlerFunc: func(c *gin.Context, err error) {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		},
	}
}

func strictParamError(c *gin.Context, err error, status int) {
	code := "bad_request"
	if status >= 500 {
		code = "internal_error"
	}
	writeError(c, status, code, err.Error())
}

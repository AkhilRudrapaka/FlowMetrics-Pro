// Package respond renders the API's uniform envelopes:
//
//	success: {"data": ..., "meta": {...pagination, lists only}}
//	failure: {"error": {"code": "...", "message": "...", "details": {...}}}
package respond

import (
	"errors"
	"log/slog"
	"net/http"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/model"

	"github.com/gin-gonic/gin"
)

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

func List(c *gin.Context, data any, meta model.PageMeta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// Error renders an *apperr.Error as-is. Anything else is a bug or an outage: it is logged
// with context and the client only sees a generic 500, so internals never leak.
func Error(c *gin.Context, err error) {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		slog.Error("unhandled error", "err", err, "method", c.Request.Method, "route", c.FullPath())
		ae = apperr.New(http.StatusInternalServerError, "internal_error", "something went wrong")
	}
	c.AbortWithStatusJSON(ae.Status, gin.H{"error": ae})
}

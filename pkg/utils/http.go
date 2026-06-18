package utils

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// HeaderAPIToken is the request header carrying the infrastructure API token
	HeaderAPIToken = "X-API-Token" //nolint:gosec // G101: this is the header name, not a credential
)

// RequireAPIToken returns a gin middleware that guards infrastructure endpoints
// with a shared token supplied via the HeaderAPIToken request header.
// When token is empty the middleware is a no-op pass-through, preserving the
// unauthenticated behavior for consumers that have not opted in.
func RequireAPIToken(token string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if token == "" {
			ctx.Next()
			return
		}
		apiToken := ctx.Request.Header.Get(HeaderAPIToken)
		if subtle.ConstantTimeCompare([]byte(apiToken), []byte(token)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api token"})
			return
		}
		ctx.Next()
	}
}

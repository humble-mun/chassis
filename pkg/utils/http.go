package utils

import (
	"github.com/gin-gonic/gin"
)

// APIVersion creates a versioned API router group with the given prefix and middlewares
func APIVersion(prefix string, middlewares ...gin.HandlerFunc) func(...func(*gin.RouterGroup)) func(*gin.Engine) {
	return func(apis ...func(*gin.RouterGroup)) func(*gin.Engine) {
		return func(mux *gin.Engine) {
			group := mux.Group(prefix, middlewares...)
			for _, api := range apis {
				api(group)
			}
		}
	}
}

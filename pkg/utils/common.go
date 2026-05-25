package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// Object is a singleton empty struct used as a marker value
	Object = new(struct{})
)

// RemoveIndex removes an element at the specified index from a slice
func RemoveIndex[T any](in []T, i int) []T {
	if in == nil {
		return in
	} else if count := len(in); count <= 0 {
		return in
	} else if i >= count || i < 0 {
		return in
	} else if i == count-1 {
		return in[:i]
	}
	return append(in[:i], in[i+1:]...)
}

const (
	noLogKey = "no-log"
)

// IsNoLog checks if the request should skip logging
func IsNoLog(ctx *gin.Context) bool {
	_, exist := ctx.Get(noLogKey)
	return exist
}

// NoLog is a middleware that marks a request to skip logging
func NoLog(ctx *gin.Context) {
	ctx.Set(noLogKey, Object)
	ctx.Next()
}

var (
	invalidNameChars = regexp.MustCompile(`[^a-z0-9\-.]`)
	leadingInvalid   = regexp.MustCompile(`^[^a-z0-9]+`)
	trailingInvalid  = regexp.MustCompile(`[^a-z0-9]+$`)
	consecutiveDash  = regexp.MustCompile(`-+`)
)

// NormalizeToKubernetesName converts a tag string to a valid Kubernetes resource name
func NormalizeToKubernetesName(tag string) (name string) {
	if tag == "" {
		return "unnamed"
	}

	name = strings.ToLower(tag)
	name = invalidNameChars.ReplaceAllString(name, "-")
	name = consecutiveDash.ReplaceAllString(name, "-")
	name = leadingInvalid.ReplaceAllString(name, "")
	name = trailingInvalid.ReplaceAllString(name, "")

	if name == "" {
		hash := sha256.Sum256([]byte(tag))
		name = "tag-" + hex.EncodeToString(hash[:])[:8]
		return
	}

	if len(name) > 253 {
		hash := sha256.Sum256([]byte(tag))
		hashSuffix := hex.EncodeToString(hash[:])[:8]
		name = name[:244] + "-" + hashSuffix
	}

	return
}

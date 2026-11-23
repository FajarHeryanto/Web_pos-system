package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS mengizinkan origin eksplisit POS_CORS_ORIGINS (pisahkan dengan koma),
// serta domain deployment Vercel. Header Authorization dan JSON diperbolehkan.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowedOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func allowedOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" {
		return false
	}
	if parsed.Host == "localhost" || strings.HasPrefix(parsed.Host, "localhost:") ||
		parsed.Host == "127.0.0.1" || strings.HasPrefix(parsed.Host, "127.0.0.1:") {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("POS_CORS_ORIGINS"), ",") {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	if strings.EqualFold(os.Getenv("VERCEL"), "1") || os.Getenv("VERCEL_URL") != "" {
		host := strings.ToLower(parsed.Hostname())
		return strings.HasSuffix(host, ".vercel.app")
	}
	return false
}

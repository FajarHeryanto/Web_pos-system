package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims berisi informasi user yang di-sign ke JWT.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func secret() []byte { return []byte(os.Getenv("JWT_SECRET")) }

// GenerateToken membuat JWT HS256 valid selama 12 jam.
func GenerateToken(userID, username, fullName, role string) (string, error) {
	s := secret()
	claims := Claims{
		UserID: userID, Username: username, FullName: fullName, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "pos-system",
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s)
}

// RequireAuth memeriksa Bearer token dan menaruh claims ke Gin context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := secret()
		if len(s) < 32 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"status": "error", "message": "JWT_SECRET belum disetel dengan aman",
				"data": nil, "errors": []interface{}{},
			})
			return
		}

		header := c.GetHeader("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status": "error", "message": "Token Bearer diperlukan",
				"data": nil, "errors": []interface{}{},
			})
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return s, nil
		}, jwt.WithIssuer("pos-system"))
		if err != nil || token == nil || !token.Valid || claims.UserID == "" || claims.Role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status": "error", "message": "Token tidak valid atau sudah kedaluwarsa",
				"data": nil, "errors": []interface{}{},
			})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("full_name", claims.FullName)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireAdmin hanya mengizinkan user dengan role admin.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"status": "error", "message": "Aksi ini hanya untuk admin",
				"data": nil, "errors": []interface{}{},
			})
			return
		}
		c.Next()
	}
}

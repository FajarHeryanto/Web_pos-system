package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"pos-system/go/handlers"
	"pos-system/go/middleware"
	"pos-system/go/utils"
)

// New menyusun router bersama untuk server lokal dan Vercel.
func New() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "POS API siap", "data": gin.H{"service": "pos-system"}, "errors": []interface{}{}})
	})
	r.POST("/api/auth/login", handlers.Login)

	api := r.Group("/api")
	api.Use(middleware.RequireAuth())
	api.GET("/auth/me", handlers.Me)
	api.GET("/products", handlers.ListProducts)
	api.GET("/products/:id", handlers.GetProduct)
	api.POST("/products", middleware.RequireAdmin(), handlers.CreateProduct)
	api.PUT("/products/:id", middleware.RequireAdmin(), handlers.UpdateProduct)
	api.DELETE("/products/:id", middleware.RequireAdmin(), handlers.DeleteProduct)
	api.GET("/transactions", handlers.ListTransactions)
	api.POST("/transactions", handlers.CreateTransaction)
	api.DELETE("/transactions/:id", middleware.RequireAdmin(), handlers.DeleteTransaction)
	api.POST("/process", middleware.RequireAdmin(), handlers.ProcessTransactions)
	api.POST("/reports/generate", middleware.RequireAdmin(), handlers.GenerateReport)
	api.GET("/reports/income", handlers.GetReport)
	api.GET("/dashboard", handlers.Dashboard)

	// Frontend statis: nama berkas di-allowlist, tidak menerima path bebas.
	web := utils.WebDir()
	pages := map[string]string{
		"/": "index.html", "/login": "login.html", "/login.html": "login.html",
		"/index.html": "index.html", "/products": "products.html", "/products.html": "products.html",
		"/transactions": "transactions.html", "/transactions.html": "transactions.html",
		"/reports": "reports.html", "/reports.html": "reports.html",
	}
	for route, file := range pages {
		filePath := filepath.Join(web, file)
		r.GET(route, func(c *gin.Context) {
			if strings.HasSuffix(c.Request.URL.Path, ".html") || c.Request.URL.Path == "/" {
				c.File(filePath)
				return
			}
			c.Redirect(http.StatusTemporaryRedirect, "/"+file)
		})
	}
	r.Static("/css", filepath.Join(web, "css"))
	r.Static("/js", filepath.Join(web, "js"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Endpoint tidak ditemukan", "data": nil, "errors": []interface{}{}})
			return
		}
		c.File(filepath.Join(web, "index.html"))
	})
	return r
}

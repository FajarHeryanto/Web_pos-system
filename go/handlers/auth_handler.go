package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"pos-system/go/database"
	"pos-system/go/middleware"
	"pos-system/go/models"
	"pos-system/go/utils"
)

// Login memvalidasi kredensial bcrypt dan mengembalikan JWT.
func Login(c *gin.Context) {
	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Error(c, http.StatusBadRequest, "Format JSON tidak valid", []utils.FieldError{{Field: "body", Message: "Kirim username dan password sebagai JSON"}})
		return
	}
	if errs := utils.ValidateLogin(&input); len(errs) > 0 {
		utils.Error(c, http.StatusBadRequest, "Input login tidak valid", errs)
		return
	}

	var user models.User
	err := database.DB().QueryRowContext(c.Request.Context(), `
		SELECT user_id, username, password_hash, full_name, role
		FROM users WHERE username = $1`, input.Username).
		Scan(&user.UserID, &user.Username, &user.PasswordHash, &user.FullName, &user.Role)
	if err != nil || !utils.CheckPassword(user.PasswordHash, input.Password) {
		utils.Error(c, http.StatusUnauthorized, "Username atau password salah", nil)
		return
	}

	token, err := middleware.GenerateToken(user.UserID, user.Username, user.FullName, user.Role)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal membuat token login", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Login berhasil", gin.H{
		"token": token,
		"user": gin.H{
			"user_id": user.UserID, "username": user.Username,
			"full_name": user.FullName, "role": user.Role,
		},
	})
}

// Me mengembalikan identitas yang sudah diverifikasi dari JWT.
func Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	fullName, _ := c.Get("full_name")
	role, _ := c.Get("role")
	utils.Success(c, http.StatusOK, "Profil aktif", gin.H{
		"user_id": userID, "username": username, "full_name": fullName, "role": role,
	})
}


package models

// User merepresentasikan satu baris tabel users.
type User struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	FullName     string `json:"full_name"`
	Role         string `json:"role"`
}

// LoginInput adalah body JSON untuk POST /api/auth/login.
type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

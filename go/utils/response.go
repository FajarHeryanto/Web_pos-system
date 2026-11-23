package utils

import "github.com/gin-gonic/gin"

// FieldError menjelaskan kesalahan validasi pada satu field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Success mengirim response JSON standar untuk hasil sukses.
func Success(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, gin.H{
		"status":  "success",
		"message": message,
		"data":    data,
		"errors":  []FieldError{},
	})
}

// Error mengirim response JSON standar untuk kegagalan dan menghentikan handler berikutnya.
func Error(c *gin.Context, code int, message string, errs []FieldError) {
	if errs == nil {
		errs = []FieldError{}
	}
	c.AbortWithStatusJSON(code, gin.H{
		"status":  "error",
		"message": message,
		"data":    nil,
		"errors":  errs,
	})
}

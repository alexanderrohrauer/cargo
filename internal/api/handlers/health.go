package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck handles GET /api/v1/health and returns a simple status response.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

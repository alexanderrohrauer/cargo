package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"cargo/internal/sops"
)

// GetAgePublicKey handles GET /api/v1/key/age and returns the server's age public key.
func GetAgePublicKey(workdir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ageKeyPath := filepath.Join(workdir, "age.key")
		pubKey, err := sops.ReadPublicKey(ageKeyPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"public_key": pubKey})
	}
}

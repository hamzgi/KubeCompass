package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hamzgi/KubeCompass/services/api/internal/cluster"
)

func main() {
	client, clientErr := cluster.NewClient()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{http.MethodGet},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/healthz", func(c *gin.Context) {
		if clientErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "clusterConnected": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "clusterConnected": true})
	})

	router.GET("/api/v1/summary", func(c *gin.Context) {
		if clientErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Kubernetes cluster is not reachable",
			})
			return
		}

		summary, err := client.Summary(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Unable to read cluster resources"})
			return
		}

		c.JSON(http.StatusOK, summary)
	})

	listenAddress := os.Getenv("API_LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":8080"
	}

	_ = router.Run(listenAddress)
}

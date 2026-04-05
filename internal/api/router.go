package api

import (
	"net/http"
	"strings"
	"time"

	"broadcast/internal/clip"
	"broadcast/internal/config"
	"broadcast/internal/media"
	"broadcast/internal/model"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	mediaSvc := media.NewService(cfg.FFmpegBin, cfg.FFprobeBin, cfg.WorkDir)
	clipSvc := clip.NewService(cfg.FFmpegBin, cfg.WorkDir, mediaSvc)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"service":    "broadcast-local-backend",
			"env":        cfg.AppEnv,
			"serverTime": time.Now().Format(time.RFC3339),
		})
	})

	r.POST("/v1/video/clip", func(c *gin.Context) {
		var req model.ClipRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "detail": err.Error()})
			return
		}
		req.VideoPath = strings.TrimSpace(req.VideoPath)
		if req.VideoPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "detail": "video_path is required"})
			return
		}

		out, err := clipSvc.ClipVideo(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "clip video failed", "detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	})

	return r
}

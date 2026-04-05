package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// App holds runtime configuration loaded from environment variables.
type App struct {
	ServerPort             string
	AppEnv                 string
	LogLevel               string
	WorkDir                string
	FFmpegBin              string
	FFprobeBin             string
	ASRBaseURL             string
	ASRAPIKey              string
	ASRModel               string
	ASRChunkSeconds        int
	ASRChunkOverlapSeconds int
	ASRRetryCount          int
}

func Load() (App, error) {
	cfg := App{
		ServerPort:             getEnv("SERVER_PORT", "8088"),
		AppEnv:                 getEnv("APP_ENV", "dev"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		WorkDir:                getEnv("WORK_DIR", "./data"),
		FFmpegBin:              getEnv("FFMPEG_BIN", "ffmpeg"),
		FFprobeBin:             getEnv("FFPROBE_BIN", "ffprobe"),
		ASRBaseURL:             getEnv("ASR_BASE_URL", "https://api.openai.com"),
		ASRAPIKey:              getEnv("ASR_API_KEY", ""),
		ASRModel:               getEnv("ASR_MODEL", "whisper-1"),
		ASRChunkSeconds:        getEnvAsInt("ASR_CHUNK_SECONDS", 600),
		ASRChunkOverlapSeconds: getEnvAsInt("ASR_CHUNK_OVERLAP_SECONDS", 2),
		ASRRetryCount:          getEnvAsInt("ASR_RETRY_COUNT", 3),
	}

	if _, err := strconv.Atoi(cfg.ServerPort); err != nil {
		return App{}, fmt.Errorf("invalid SERVER_PORT %q: %w", cfg.ServerPort, err)
	}

	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return App{}, fmt.Errorf("invalid LOG_LEVEL %q: allowed debug|info|warn|error", cfg.LogLevel)
	}

	cfg.ASRBaseURL = strings.TrimRight(cfg.ASRBaseURL, "/")
	if cfg.ASRBaseURL == "" {
		return App{}, fmt.Errorf("ASR_BASE_URL is required")
	}
	if cfg.ASRAPIKey == "" {
		return App{}, fmt.Errorf("ASR_API_KEY is required")
	}
	if cfg.ASRModel == "" {
		return App{}, fmt.Errorf("ASR_MODEL is required")
	}
	if cfg.ASRChunkSeconds <= 0 {
		return App{}, fmt.Errorf("ASR_CHUNK_SECONDS must be > 0")
	}
	if cfg.ASRChunkOverlapSeconds < 0 {
		return App{}, fmt.Errorf("ASR_CHUNK_OVERLAP_SECONDS must be >= 0")
	}
	if cfg.ASRChunkOverlapSeconds >= cfg.ASRChunkSeconds {
		return App{}, fmt.Errorf("ASR_CHUNK_OVERLAP_SECONDS must be < ASR_CHUNK_SECONDS")
	}
	if cfg.ASRRetryCount <= 0 {
		return App{}, fmt.Errorf("ASR_RETRY_COUNT must be > 0")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return n
}

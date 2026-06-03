package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort   string
	UploadDir    string
	TempDir      string
	MaxUploadMB  int64
	AllowedExts  []string
	
	// OpenCV параметры
	CannyThreshold1 int
	CannyThreshold2 int
	HoughThreshold  int
	MinLineLength   float64
	MaxLineGap      float64

	// Таймауты
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Load() *Config {
	maxUpload, _ := strconv.ParseInt(getEnv("MAX_UPLOAD_MB", "50"), 10, 64)
	
	return &Config{
		ServerPort:   getEnv("SERVER_PORT", ":8080"),
		UploadDir:    getEnv("UPLOAD_DIR", "./uploads"),
		TempDir:      getEnv("TEMP_DIR", "./tmp"),
		MaxUploadMB:  maxUpload,
		AllowedExts:  []string{".mp4", ".mov", ".avi", ".jpg", ".png"},
		
		CannyThreshold1: getEnvInt("CANNY_THRESH1", 50),
		CannyThreshold2: getEnvInt("CANNY_THRESH2", 150),
		HoughThreshold:  getEnvInt("HOUGH_THRESH", 100),
		MinLineLength:   getEnvFloat("MIN_LINE_LEN", 30.0),
		MaxLineGap:      getEnvFloat("MAX_LINE_GAP", 10.0),

		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return fallback
}

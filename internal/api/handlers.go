package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gocv.io/x/gocv"

	"roomscan-ai/internal/config"
	"roomscan-ai/internal/worker"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: В продакшене заменить 
		return r.Header.Get("Origin") == "http://localhost:3000" || r.Header.Get("Origin") == ""
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	cfg    *config.Config
	worker *worker.Processor
}

func NewHandler(cfg *config.Config, worker *worker.Processor) *Handler {
	return &Handler{cfg: cfg, worker: worker}
}

// HandleUpload обрабатывает загрузку файла с валидацией
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ограничение размера запроса (защита от DoS)
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadMB*1024*1024)
	if err := r.ParseMultipartForm(h.cfg.MaxUploadMB * 1024 * 1024); err != nil {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Валидация расширения
	ext := strings.ToLower(filepath.Ext(header.Filename))
	isAllowed := false
	for _, allowed := range h.cfg.AllowedExts {
		if ext == allowed {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		http.Error(w, "Invalid file type", http.StatusUnsupportedMediaType)
		return
	}

	// Генерация безопасного имени файла
	fileID := uuid.New().String()
	dst := filepath.Join(h.cfg.UploadDir, fileID+ext)

	out, err := os.Create(dst)
	if err != nil {
		slog.Error("Failed to create file", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		os.Remove(dst) // Откат при ошибке
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Запуск обработки в фоне
	go func(id, path string) {
		if err := h.worker.ProcessVideo(id, path); err != nil {
			slog.Error("Background processing failed", "id", id, "error", err)
		}
	}(fileID, dst)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     fileID,
		"status": "processing",
	})
}

// HandleARStream Исправлена утечка памяти!
func (h *Handler) HandleARStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	id := chi.URLParam(r, "id")
	slog.Info("AR Stream started", "id", id)

	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket error", "error", err)
			}
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		// ИСПРАВЛЕНИЕ: Обработка в анонимной функции или явный Close() в конце итерации,
		// а НЕ defer внутри цикла!
		func() {
			img, err := gocv.IMDecode(msg, gocv.IMReadColor)
			if err != nil {
				slog.Warn("Failed to decode image", "error", err)
				return
			}
			defer img.Close() // Теперь defer сработает при выходе из этой анонимной функции (одной итерации)

			// Пример обработки: преобразование в оттенки серого и обратно (заглушка для AR)
			gray := gocv.NewMat()
			defer gray.Close()
			gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)
			
			processed := gocv.NewMat()
			defer processed.Close()
			gocv.CvtColor(gray, &processed, gocv.ColorGrayToBGR)

			// Кодирование обратно в JPEG для отправки
			buf, err := gocv.IMEncode(".jpg", processed)
			if err != nil {
				return
			}
			defer buf.Close()

			// Отправка результата
			if err := conn.WriteMessage(websocket.BinaryMessage, buf.GetBytes()); err != nil {
				slog.Error("Failed to write WebSocket message", "error", err)
			}
		}()
	}
}

// HandleStatus возвращает статус обработки
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	status, err := h.worker.GetStatus(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     id,
		"status": status,
	})
}

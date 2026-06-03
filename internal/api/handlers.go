package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gocv.io/x/gocv"

	"github.com/AleksKAG/RoomScan-AI/internal/config"
	"github.com/AleksKAG/RoomScan-AI/internal/db"
	"github.com/AleksKAG/RoomScan-AI/internal/queue"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "http://localhost:3000" || r.Header.Get("Origin") == ""
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	cfg         *config.Config
	queueClient *queue.Client
	db          *db.Store
}

func NewHandler(cfg *config.Config, queueClient *queue.Client, db *db.Store) *Handler {
	return &Handler{cfg: cfg, queueClient: queueClient, db: db}
}

// HandleUpload godoc
// @Summary Upload a video or image for room scanning
// @Description Accepts a file, validates it, saves it, and enqueues a processing task.
// @Tags scans
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Video or Image file (mp4, mov, avi, jpg, png)"
// @Success 202 {object} map[string]string "Task enqueued"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 413 {object} map[string]string "File too large"
// @Failure 429 {object} map[string]string "Too many requests"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/v1/upload [post]
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadMB*1024*1024)
	if err := r.ParseMultipartForm(h.cfg.MaxUploadMB * 1024 * 1024); err != nil {
		http.Error(w, `{"error": "File too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error": "Failed to get file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".jpg": true, ".png": true}
	if !allowedExts[ext] {
		http.Error(w, `{"error": "Invalid file type"}`, http.StatusUnsupportedMediaType)
		return
	}

	fileID := uuid.New().String()
	dst := filepath.Join(h.cfg.UploadDir, fileID+ext)

	out, err := os.Create(dst)
	if err != nil {
		slog.Error("Failed to create file", "error", err)
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(dst)
		http.Error(w, `{"error": "Failed to save file"}`, http.StatusInternalServerError)
		return
	}
	out.Close()

	scan := &db.Scan{
		ID:       fileID,
		Status:   db.StatusQueued,
		FilePath: dst,
	}
	if err := h.db.CreateScan(r.Context(), scan); err != nil {
		slog.Error("Failed to create scan record in DB", "error", err)
		os.Remove(dst)
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}

	if err := h.queueClient.EnqueueProcessVideo(fileID, dst); err != nil {
		h.db.UpdateStatus(r.Context(), fileID, db.StatusFailed, "", err.Error())
		http.Error(w, `{"error": "Failed to start processing"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     fileID,
		"status": string(db.StatusQueued),
	})
}

// HandleStatus godoc
// @Summary Get processing status of a scan
// @Description Returns the current status of the room scan processing task.
// @Tags scans
// @Produce json
// @Param id path string true "Scan ID"
// @Success 200 {object} db.Scan
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/v1/status/{id} [get]
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	scan, err := h.db.GetScan(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scan)
}

// HandleARStream godoc
// @Summary WebSocket stream for AR processing
// @Description Receives image frames and returns processed AR frames.
// @Tags ar
// @Param id path string true "Scan ID"
// @Router /api/v1/ws/ar/{id} [get]
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

		// ИСПРАВЛЕНИЕ УТЕЧКИ: Анонимная функция гарантирует вызов defer на каждой итерации
		func() {
			img, err := gocv.IMDecode(msg, gocv.IMReadColor)
			if err != nil {
				return
			}
			defer img.Close()

			gray := gocv.NewMat()
			defer gray.Close()
			gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)
			
			processed := gocv.NewMat()
			defer processed.Close()
			gocv.CvtColor(gray, &processed, gocv.ColorGrayToBGR)

			buf, err := gocv.IMEncode(".jpg", processed)
			if err != nil {
				return
			}
			defer buf.Close()

			if err := conn.WriteMessage(websocket.BinaryMessage, buf.GetBytes()); err != nil {
				slog.Error("Failed to write WebSocket message", "error", err)
			}
		}()
	}
}

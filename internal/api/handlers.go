package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/yourname/roomscan/internal/ar"
	"github.com/yourname/roomscan/internal/design"
	"github.com/yourname/roomscan/internal/worker"
	"gocv.io/x/gocv"
)

const uploadDir = "/tmp/roomscan_uploads"

func init() {
	_ = os.MkdirAll(uploadDir, 0755)
}

type uploadResponse struct {
	ID string `json:"id"`
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	
	go func(id, path string) {
		log.Printf("[worker] start processing %s\n", id)
		if err := worker.ProcessVideo(id, path); err != nil {
			log.Printf("error: %v", err)
		}
	}(id, dst)
	
}

// resultHandler: возвращает JSON с файлами
func resultHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	resultsDir := filepath.Join(uploadDir, id+"_results")
	statusFile := filepath.Join(resultsDir, "status.json")
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		http.Error(w, "result not ready", http.StatusNotFound)
		return
	}

	response := map[string]string{
		"plan":  filepath.Join("/results", id, "plan.png"),
		"gltf":  filepath.Join("/results", id, "model.gltf"),
		"pdf":   filepath.Join("/results", id, "report.pdf"),
	}
	json.NewEncoder(w).Encode(response)
}

func generateDesignHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	style := r.FormValue("style")
	resultsDir := filepath.Join(uploadDir, id+"_results")
	planPath := filepath.Join(resultsDir, "plan.png")

	paths, err := design.GenerateWithKandinsky(planPath, style) // или GigaChat
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string][]string{"designs": paths})
}

// arStreamHandler (для реал-тайм AR)
func arStreamHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// msg — JPEG frame от клиента
		img, _, err := gocv.IMDecode(msg, gocv.IMReadColor)
		if err != nil {
			continue
		}
		defer img.Close()

		// Оцифровка
		poly := ar.ScanRoom(img)

		// TODO: Генерация дизайна если выбран стиль (из query param или session)

		// Наложение маски (placeholder: инверт цвета для теста)
		designImg := img.Clone() // Замените на generated
		mask := gocv.NewMat()    // TODO: segmentation mask
		overlaid := ar.ApplyOverlay(img, designImg, mask)

		// Отправка back
		buf, err := gocv.IMEncode(".jpg", overlaid)
		if err != nil {
			continue
		}
		conn.WriteMessage(websocket.BinaryMessage, buf.GetBytes())
	}
}

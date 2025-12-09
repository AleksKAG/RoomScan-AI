package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)
// Вариант 1: Kandinsky 3.1 (бесплатно)
// ----------------------
type KandinskyRequest struct {
	ModelID    string            `json:"model_id"`
	Params     KandinskyParams   `json:"params"`
}

type KandinskyParams struct {
	Type        string   `json:"type"`
	Text        string   `json:"text"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Negative    string   `json:"negativePrompt,omitempty"`
	NumImages   int      `json:"num_images,omitempty"`
}

type KandinskyImage struct {
	UUID string `json:"uuid"`
}

type KandinskyResponse struct {
	Images []KandinskyImage `json:"images"`
}

// GenerateWithKandinsky — бесплатная генерация через Kandinsky 3.1
func GenerateWithKandinsky(planImagePath string, style string) ([]string, error) {
	// Читаем план комнаты как base64 (Kandinsky поддерживает image-to-image)
	imgBytes, _ := os.ReadFile(planImagePath)
	// Здесь можно добавить base64, но для MVP просто используем текстовый промпт

	prompt := fmt.Sprintf(
		"Современный интерьер пустой комнаты по этому плану, стиль: %s, высокое качество, реалистично, красивое освещение, 4K",
		style,
	)

	reqBody := KandinskyRequest{
		ModelID: "v3", // или "v3-fusion"
		Params: KandinskyParams{
			Type:      "generate",
			Text:      prompt,
			Width:     1024,
			Height:    768,
			NumImages: 4,
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post("https://api.kandinsky.ai/v3/generate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Images []struct {
				Base64 string `json:"base64"`
			} `json:"images"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	var outputPaths []string
	outDir := filepath.Dir(planImagePath)

	for i, img := range result.Result.Images {
		data, _ := base64.StdEncoding.DecodeString(img.Base64)
		path := filepath.Join(outDir, fmt.Sprintf("design_%d.jpg", i+1))
		os.WriteFile(path, data, 0644)
		outputPaths = append(outputPaths, path)
	}

	return outputPaths, nil
}

// ----------------------
// Вариант 2: GigaChat Pro Vision 
// ----------------------
func GenerateWithGigaChat(planImagePath string, style string) ([]string, error) {
	// Нужно получить токен через https://developers.sber.ru
	// Пример запроса (упрощённо):
	payload := map[string]interface{}{
		"model": "GigaChat-Pro",
		"messages": []map[string]string{
			{
				"role": "user",
				"content": fmt.Sprintf("Сделай красивый дизайн этой комнаты в стиле: %s. Верни 4 варианта.", style),
			},
		},
		"images": []string{ "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(...)},
	}

	// Реализация по аналогии с Kandinsky
	// Возвращаем пути к сгенерированным изображениям
}

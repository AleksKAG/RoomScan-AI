package design

import (
	"context"
	"log/slog"
)

// Generator отвечает за генерацию дизайнов с помощью AI
type Generator struct {
	// TODO: Добавить поля для API клиентов
}

// NewGenerator создает новый экземпляр генератора
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateDesign генерирует описание дизайна на основе геометрии комнаты
func (g *Generator) GenerateDesign(ctx context.Context, roomType string, polygonJSON string) (string, error) {
	slog.Info("Generating design", "room_type", roomType, "polygon", polygonJSON)
	
	// TODO: Реализовать интеграцию с GigaChat или другой LLM
	// Временно возвращаем заглушку
	return "Modern minimalist interior design with light colors and clean lines", nil
}

// GenerateWithGigaChat - устаревший метод, оставлен для совместимости
func GenerateWithGigaChat(prompt string) (string, error) {
	slog.Info("GenerateWithGigaChat called", "prompt", prompt)
	return "Mock design description: Modern minimalist interior with light colors", nil
}

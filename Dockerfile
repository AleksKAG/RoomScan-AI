# ==================== BUILD STAGE ====================
FROM golang:1.22-bookworm AS builder

# Устанавливаем зависимости OpenCV + ffmpeg
RUN apt-get update && apt-get install -y \
    libopencv-dev \
    ffmpeg \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем go.mod и скачиваем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем с CGO (нужно для gocv)
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /RoomScan-AI ./cmd/api/main.go

# ==================== RUNTIME STAGE ====================
FROM debian:bookworm-slim

# Устанавливаем runtime-библиотеки (без dev-пакетов)
RUN apt-get update && apt-get install -y \
    libopencv4.7 \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Создаём директорию для загрузок
RUN mkdir -p /tmp/RoomScan-AI_uploads

WORKDIR /app

# Копируем только бинарник из builder
COPY --from=builder /RoomScan-AI /RoomScan-AI

EXPOSE 8080

# Запуск
CMD ["/RoomScan-AI"]

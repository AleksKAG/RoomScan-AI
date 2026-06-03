# --- Этап 1: Сборка ---
FROM golang:1.22-bullseye AS builder

# Установка зависимостей для gocv (OpenCV)
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libv4l-dev \
    libxvidcore-dev \
    libx264-dev \
    libgtk-3-dev \
    libatlas-base-dev \
    gfortran \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Кэширование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка бинарного файла (статическая линковка предпочтительна, но для OpenCV нужны динамические библиотеки)
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o roomscan .

# --- Этап 2: Рантайм ---
FROM debian:bullseye-slim

# Установка только необходимых runtime-зависимостей для OpenCV
RUN apt-get update && apt-get install -y \
    libjpeg62-turbo \
    libpng16-16 \
    libtiff5 \
    libavcodec58 \
    libavformat58 \
    libswscale5 \
    libgtk-3-0 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Создание non-root пользователя для безопасности
RUN useradd -m -u 1000 roomscan
USER roomscan

# Копирование бинарника из builder
COPY --from=builder /app/roomscan .

# Создание директорий для файлов с правильными правами
RUN mkdir -p /app/uploads /app/tmp && chown roomscan:roomscan /app/uploads /app/tmp

ENV SERVER_PORT=:8080
ENV UPLOAD_DIR=/app/uploads
ENV TEMP_DIR=/app/tmp

EXPOSE 8080

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["./roomscan"]

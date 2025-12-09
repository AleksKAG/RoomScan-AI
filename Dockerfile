FROM golang:1.22-buster AS build
RUN apt-get update && apt-get install -y libopencv-dev ffmpeg
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=1 go build -o /roomscan cmd/api/main.go

FROM debian:buster-slim
RUN apt-get update && apt-get install -y libopencv4.2 ffmpeg ca-certificates
COPY --from=build /roomscan /roomscan
EXPOSE 8080
CMD ["/roomscan"]

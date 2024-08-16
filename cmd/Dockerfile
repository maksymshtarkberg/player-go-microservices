# Используем официальный образ Go в качестве базового
FROM golang:1.22.2 AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN go build -o main ./cmd

# Используем лёгкий образ для запуска приложения
FROM alpine:latest

WORKDIR /root/

# Копируем скомпилированное приложение из стадии сборки
COPY --from=builder /app/main .

# Открываем порт
EXPOSE 8080

# Команда для запуска приложения
CMD ["./main"]

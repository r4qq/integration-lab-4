FROM golang:1.25.9-alpine
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
cmd ["go", "run", "cmd/api/main.go"]
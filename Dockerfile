# 使用官方 Go 映像作為建構階段
FROM golang:1.24.1-alpine AS builder

# 設定工作目錄
WORKDIR /app

# 安裝必要的依賴
RUN apk add --no-cache git ca-certificates tzdata

# 複製 go mod 檔案
COPY go.mod go.sum ./

# 下載依賴
RUN go mod download

# 複製源代碼
COPY . .

# 編譯應用程式
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o better-myUT main.go

# 使用最小的 Alpine 映像作為執行階段
FROM alpine:latest

# 安裝 ca-certificates 以支援 HTTPS
RUN apk --no-cache add ca-certificates tzdata

# 設定工作目錄
WORKDIR /root/

# 從建構階段複製編譯好的程式
COPY --from=builder /app/better-myUT .

# 複製環境變數範例檔案
COPY --from=builder /app/env.example .

# 設定時區為台北
ENV TZ=Asia/Taipei

# 暴露端口
EXPOSE 8080

# 執行程式
CMD ["./better-myUT"] 
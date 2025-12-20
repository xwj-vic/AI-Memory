# ========== 第一阶段：构建前端 ==========
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ========== 第二阶段：构建后端 ==========
FROM golang:1.24-alpine AS backend-builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ai-memory

# ========== 第三阶段：运行 ==========
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 复制二进制和前端
COPY --from=backend-builder /app/ai-memory .
COPY --from=backend-builder /app/frontend/dist ./frontend/dist

# 创建日志目录
RUN mkdir -p /app/log

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./ai-memory"]

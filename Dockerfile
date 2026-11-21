# 构建Go应用
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

# 安装GCC和相关构建工具（webp包需要CGO支持）
RUN apk add --no-cache gcc musl-dev

# 复制Go项目文件
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 构建Go应用（需要CGO支持webp包）
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# 最终运行镜像
FROM alpine:latest

WORKDIR /app

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 从构建阶段复制构建产物
COPY --from=backend-builder /app/main /app/

# 暴露应用端口
EXPOSE 3000

# 运行应用
CMD ["./main"]

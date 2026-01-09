# 使用官方 Go 镜像作为构建环境
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o douban-timeline .

# 使用轻量级镜像作为运行环境
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add \
    ca-certificates \
    python3 \
    py3-pip \
    && python3 -m pip install --break-system-packages httpx aiofiles lxml

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/douban-timeline .

# 创建必要的目录
RUN mkdir -p /app/images /app/templates /app/static

# 复制模板和静态文件
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# 复制 Python 爬虫脚本
COPY --from=builder /app/javbus_crawler.py .

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./douban-timeline"]


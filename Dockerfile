# 第一阶段：编译环境 (使用与你本地一致的 1.24 版本)
FROM golang:1.24.5-alpine AS builder

WORKDIR /app
# 开启 Go Module 并设置国内代理，加速依赖下载
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# 先复制 mod 文件并下载依赖，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源代码
COPY . .

# 编译成可执行文件 (静态编译，脱离动态库依赖)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/overseer ./cmd/ascentia-core

# 第二阶段：极简运行环境 (Alpine 只有 5MB 大小)
FROM alpine:latest

# 安装时区和基础证书库（大模型 HTTPS 请求必备）
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从第一阶段把编译好的二进制文件拿过来
COPY --from=builder /app/overseer .

# 暴露 8080 端口
EXPOSE 8080

# 启动微服务
CMD ["./overseer"]
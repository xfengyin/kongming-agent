# ---- 依赖阶段 ----
FROM golang:1.21-alpine AS builder

# 安装构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 先复制 go.mod 和 go.sum 以利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o kongming \
    ./cmd/kongming

# ---- 运行阶段 ----
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN adduser -D -g '' appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/kongming .
COPY --from=builder /app/configs ./configs

# 复制健康检查脚本
COPY --from=builder /app/scripts/healthcheck.sh /usr/local/bin/

# 更改文件所有者
RUN chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD /usr/local/bin/healthcheck.sh || exit 1

# 启动命令
ENTRYPOINT ["./kongming"]
CMD ["--config", "./configs/kongming.yaml"]

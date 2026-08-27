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

# 从构建阶段复制二进制与示例知识库
COPY --from=builder /app/kongming .
COPY --from=builder /app/knowledge ./knowledge
COPY --from=builder /app/config.example.yaml .

# 更改文件所有者
RUN chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 一次性离线 demo（CLI 是 REPL，容器非 -it 下 stdin EOF 立即退出，故用 --ask 单轮）
# 交互模式：docker run -it --rm zhuge/kongming:latest --mock --interactive
ENTRYPOINT ["./kongming"]
CMD ["--mock", "--knowledge", "./knowledge", "--ask", "司马懿兵临城下，如何用空城计退敌？"]

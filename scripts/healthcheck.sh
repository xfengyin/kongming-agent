#!/bin/sh
# Healthcheck script for Kongming

# 检查进程是否存在
if ! pgrep -x "kongming" > /dev/null; then
    exit 1
fi

# 检查健康端点
if command -v curl > /dev/null; then
    if ! curl -sf http://localhost:9090/health > /dev/null; then
        exit 1
    fi
fi

exit 0

#!/bin/bash

# 设置UTF-8编码
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8

# 跨平台编译脚本
echo "开始跨平台编译..."

# 创建releases目录（如果不存在）
RELEASES_DIR="releases"
if [ ! -d "$RELEASES_DIR" ]; then
    echo "创建releases目录..."
    mkdir -p "$RELEASES_DIR"
fi

# Windows x64
echo "编译 Windows x64..."
GOOS=windows GOARCH=amd64 go build -o "$RELEASES_DIR/cyberscan_windows_x64.exe" ./cmd/main.go

# Linux x64
echo "编译 Linux x64..."
GOOS=linux GOARCH=amd64 go build -o "$RELEASES_DIR/cyberscan_linux_x64" ./cmd/main.go

# macOS ARM64 (Apple Silicon)
echo "编译 macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -o "$RELEASES_DIR/cyberscan_darwin_arm64" ./cmd/main.go

# macOS x64 (Intel)
echo "编译 macOS x64..."
GOOS=darwin GOARCH=amd64 go build -o "$RELEASES_DIR/cyberscan_darwin_x64" ./cmd/main.go

echo "编译完成！生成的文件："
ls -la "$RELEASES_DIR"/cyberscan_*

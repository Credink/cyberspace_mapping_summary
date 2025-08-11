@echo off
chcp 65001 >nul
echo 开始跨平台编译...

REM 创建releases目录（如果不存在）
if not exist "releases" (
    echo 创建releases目录...
    mkdir releases
)

echo 编译 Windows x64...
set GOOS=windows
set GOARCH=amd64
go build -o releases\cyberscan_windows_x64.exe ./cmd/main.go

echo 编译 Linux x64...
set GOOS=linux
set GOARCH=amd64
go build -o releases\cyberscan_linux_x64 ./cmd/main.go

echo 编译 macOS ARM64...
set GOOS=darwin
set GOARCH=arm64
go build -o releases\cyberscan_darwin_arm64 ./cmd/main.go

echo 编译 macOS x64...
set GOOS=darwin
set GOARCH=amd64
go build -o releases\cyberscan_darwin_x64 ./cmd/main.go

echo 编译完成！
dir releases\cyberscan_*

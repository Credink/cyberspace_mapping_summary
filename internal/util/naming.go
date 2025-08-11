package util

import (
	"fmt"
	"time"
)

// GenerateTaskID 生成统一的任务ID
func GenerateTaskID() string {
	now := time.Now()
	dateStr := now.Format("20060102")
	tsStr := fmt.Sprintf("%d", now.Unix())
	shortTS := tsStr[len(tsStr)-8:]
	return fmt.Sprintf("%s_%s", dateStr, shortTS)
}

// GenerateTableName 生成数据库表名
func GenerateTableName(taskID string) string {
	return fmt.Sprintf("task_%s", taskID)
}

// GenerateCSVFileName 生成CSV文件名
func GenerateCSVFileName(taskID, suffix string) string {
	return fmt.Sprintf("%s_%s.csv", taskID, suffix)
}

// GenerateTXTFileName 生成TXT文件名
func GenerateTXTFileName(taskID, suffix string) string {
	return fmt.Sprintf("%s_%s.txt", taskID, suffix)
}

// GenerateProjectDir 生成项目目录名
func GenerateProjectDir(baseDir string) string {
	taskID := GenerateTaskID()
	return fmt.Sprintf("%s_%s", taskID[:8], taskID[9:]) // 格式：20250726_155103
}

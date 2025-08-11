package query

import (
	"cyberspace_mapping_summary/internal/model"
	"fmt"
	"strings"
	"time"
)

// isRetryableError 判断是否为可重试的错误
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// 检查各种API限制错误
	retryableErrors := []string{
		"请求太多",
		"稍后再试",
		"rate limit",
		"too many requests",
		"quota exceeded",
		"api limit",
		"请求频率过高",
		"请求过于频繁",
		"请求超限",
		"请求限制",
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(errMsg, retryableErr) {
			return true
		}
	}

	return false
}

// retryWithBackoff 带退避的重试函数
func retryWithBackoff(platform, target string, queryFunc func() ([]model.QueryResult, error)) ([]model.QueryResult, error) {
	var lastErr error

	// 最多重试3次
	for attempt := 1; attempt <= 3; attempt++ {
		// 执行查询
		results, err := queryFunc()

		// 如果没有错误，直接返回结果
		if err == nil {
			if attempt > 1 {
				fmt.Printf("[%s] 重试成功: %s (第%d次重试)\n", platform, target, attempt-1)
			}
			return results, nil
		}

		lastErr = err

		// 检查是否为可重试的错误
		if !isRetryableError(err) {
			fmt.Printf("[%s] 不可重试错误: %s -> %v\n", platform, target, err)
			return nil, err
		}

		// 计算重试延迟时间（递增：1轮、2轮、3轮）
		delay := time.Duration(attempt) * 3 * time.Second

		fmt.Printf("[%s] 第%d次重试失败: %s -> %v\n", platform, attempt, target, err)
		fmt.Printf("[%s] 等待%d秒后重试: %s\n", platform, delay/time.Second, target)

		// 等待后重试
		time.Sleep(delay)
	}

	// 所有重试都失败了
	fmt.Printf("[%s] 重试失败，跳过查询: %s -> %v\n", platform, target, lastErr)
	return nil, fmt.Errorf("重试%d次后仍然失败: %w", 3, lastErr)
}

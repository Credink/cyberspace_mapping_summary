package query

import (
	"net"
	"strconv"
	"strings"
)

// stringFromAny 助手函数，interface{}转string
func stringFromAny(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

// intFromAny 助手函数，接口转int
func intFromAny(value interface{}) int {
	if v, ok := value.(float64); ok {
		return int(v)
	}
	return 0
}

// extractHostAndPort 从URL或host中提取域名和端口号
func extractHostAndPort(urlOrHost string) (string, int) {
	if urlOrHost == "" {
		return "", 0
	}

	var withoutProtocol string

	// 如果包含协议（如http://或https://），则移除协议部分
	if strings.HasPrefix(urlOrHost, "http://") || strings.HasPrefix(urlOrHost, "https://") {
		withoutProtocol = strings.TrimPrefix(strings.TrimPrefix(urlOrHost, "https://"), "http://")
	} else {
		withoutProtocol = urlOrHost
	}

	// 提取域名部分（去除路径）
	if slashIndex := strings.Index(withoutProtocol, "/"); slashIndex != -1 {
		withoutProtocol = withoutProtocol[:slashIndex]
	}

	// 检查是否包含端口号
	if colonIndex := strings.Index(withoutProtocol, ":"); colonIndex != -1 {
		host := withoutProtocol[:colonIndex]
		portPart := withoutProtocol[colonIndex+1:]

		// 验证端口号是否为数字
		if port, err := strconv.Atoi(portPart); err == nil && port > 0 && port <= 65535 {
			return host, port
		}
	}

	// 如果没有端口号，返回域名和0
	return withoutProtocol, 0
}

// isIPOrCIDR 判断是否为IP地址或CIDR格式
func isIPOrCIDR(target string) bool {
	// 检查是否为CIDR格式（包含/）
	if strings.Contains(target, "/") {
		_, _, err := net.ParseCIDR(target)
		return err == nil
	}

	// 检查是否为普通IP地址
	return net.ParseIP(target) != nil
}

// isCIDR 判断是否为CIDR格式
func isCIDR(target string) bool {
	if strings.Contains(target, "/") {
		_, _, err := net.ParseCIDR(target)
		return err == nil
	}
	return false
}

// isIP 判断是否为普通IP地址（不包含CIDR）
func isIP(target string) bool {
	// 如果不是CIDR，再检查是否为IP
	if !isCIDR(target) {
		return net.ParseIP(target) != nil
	}
	return false
}

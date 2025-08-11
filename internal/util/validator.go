package util

import (
	"net"
	"regexp"
	"strings"
)

var (
	// 域名匹配正则（RFC 1035 简化版）
	domainRegexp = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

	// 私有 IP 网段（内网）
	privateCIDRs = []*net.IPNet{
		mustCIDR("10.0.0.0/8"),
		mustCIDR("172.16.0.0/12"),
		mustCIDR("192.168.0.0/16"),
		mustCIDR("127.0.0.0/8"),    // Loopback
		mustCIDR("169.254.0.0/16"), // Link-local
	}
)

// mustCIDR 用于初始化 IP 网段
func mustCIDR(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("invalid CIDR: " + cidr)
	}
	return ipnet
}

// isPrivateIP 判断是否为内网 IP
func isPrivateIP(ip net.IP) bool {
	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// IsValidHost 验证是否为合法公网 IP 或合法域名
func IsValidHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}

	// 排除 URL 或路径（但不排除CIDR格式）
	if strings.HasPrefix(host, "http") {
		return false
	}

	// 检查是否为CIDR格式
	if strings.Contains(host, "/") {
		_, ipnet, err := net.ParseCIDR(host)
		if err != nil {
			return false // 无效的CIDR格式
		}
		// 检查CIDR的IP是否为公网IP
		ip := ipnet.IP
		if ip.To4() == nil {
			return false // 排除 IPv6
		}
		if isPrivateIP(ip) {
			return false // 排除内网 IP
		}
		return true // 合法公网 CIDR
	}

	// 检查 IP（仅允许公网 IPv4）
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return false // 排除 IPv6
		}
		if isPrivateIP(ip) {
			return false // 排除内网 IP
		}
		return true // 合法公网 IP
	}

	// 检查域名
	return domainRegexp.MatchString(host)
}

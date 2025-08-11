package query

import (
	"cyberspace_mapping_summary/internal/model"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HunterConfig 用于配置API Key
type HunterConfig struct {
	APIKey string
}

// HunterAPIResponse 定义API返回结构
type HunterAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total int                      `json:"total"`
		Arr   []map[string]interface{} `json:"arr"`
	} `json:"data"`
}

// buildHunterQuery 构造Hunter查询参数
func buildHunterQuery(target, apiKey string, page, pageSize int) url.Values {
	// 判断查询类型
	isCIDR := isCIDR(target)
	isIP := isIP(target)

	// 构造查询语法
	var query string
	if isCIDR {
		// CIDR格式：ip="192.168.1.0/24"
		query = fmt.Sprintf(`ip="%s"`, target)
	} else if isIP {
		// 单个IP：ip="192.168.1.1"
		query = fmt.Sprintf(`ip="%s"`, target)
	} else {
		// 域名：domain="example.com"
		query = fmt.Sprintf(`domain="%s"`, target)
	}

	// Base64URL编码查询语法（Hunter使用base64url编码）
	queryBase64 := base64.URLEncoding.EncodeToString([]byte(query))

	params := url.Values{}
	params.Set("api-key", apiKey)
	params.Set("search", queryBase64)
	params.Set("page", strconv.Itoa(page))
	params.Set("page_size", strconv.Itoa(pageSize))
	params.Set("is_web", "3") // 3代表全部资产类型

	return params
}

// QueryHunter 单域名或IP查询接口，返回结果列表或错误
func QueryHunter(target, apiKey string) ([]model.QueryResult, error) {
	// 构造查询语法用于日志显示
	var querySyntax string
	if isCIDR(target) {
		querySyntax = fmt.Sprintf(`ip="%s"`, target)
	} else if isIP(target) {
		querySyntax = fmt.Sprintf(`ip="%s"`, target)
	} else {
		querySyntax = fmt.Sprintf(`domain="%s"`, target)
	}

	fmt.Printf("[Hunter] 开始扫描: %s (查询语法: %s)\n", target, querySyntax)

	// 使用重试机制执行查询
	return retryWithBackoff("Hunter", target, func() ([]model.QueryResult, error) {
		return queryHunterInternal(target, apiKey)
	})
}

// queryHunterInternal Hunter查询的内部实现
func queryHunterInternal(target, apiKey string) ([]model.QueryResult, error) {
	var allResults []model.QueryResult
	client := &http.Client{Timeout: 30 * time.Second}

	page := 1
	pageSize := 100 // Hunter默认每页100条

	for {
		params := buildHunterQuery(target, apiKey, page, pageSize)

		// 构造请求URL
		baseURL := "https://hunter.qianxin.com/openApi/search"
		reqURL := baseURL + "?" + params.Encode()

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("request creation failed: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response failed: %w", err)
		}

		var hunterResp HunterAPIResponse
		if err := json.Unmarshal(respBody, &hunterResp); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}

		// 检查API错误
		if hunterResp.Code != 200 {
			return nil, fmt.Errorf("hunter API error: %s", hunterResp.Message)
		}

		// 转换结果
		pageResults := 0
		for _, result := range hunterResp.Data.Arr {
			queryResult := convertHunterItemToResult("", result)
			allResults = append(allResults, queryResult)
			pageResults++
		}

		fmt.Printf("[Hunter] 第%d页扫描完成: %s -> 获得%d条结果\n", page, target, pageResults)

		// 检查是否还有更多数据
		if len(hunterResp.Data.Arr) < pageSize {
			break
		}

		page++

		// 防止无限循环
		if page > 100 {
			break
		}
	}

	fmt.Printf("[Hunter] 扫描完成: %s -> 总计%d条结果\n", target, len(allResults))
	return allResults, nil
}

// convertHunterItemToResult 转换Hunter单条结果为模型结构
func convertHunterItemToResult(unit string, item map[string]interface{}) model.QueryResult {
	// 从map中提取字段
	ip := stringFromAny(item["ip"])
	port := intFromAny(item["port"])
	protocol := stringFromAny(item["protocol"])
	title := strings.TrimSpace(strings.Trim(stringFromAny(item["web_title"]), "\n"))
	domain := stringFromAny(item["domain"])
	statusCode := intFromAny(item["status_code"])

	// Hunter API中没有length字段，使用0作为默认值
	length := 0

	// 处理host字段：Hunter API中没有host字段，使用domain作为host
	host := domain
	// 兜底策略：如果host为空，使用IP作为host
	if host == "" {
		host = ip
	}

	// 处理domain字段
	// 如果domain为空且host不是IP，则domain=host
	if domain == "" && host != "" && !isIPOrCIDR(host) {
		domain = host
	}

	// 构造URL：使用API返回的端口
	finalPort := port
	url := constructHunterURL(host, ip, finalPort, protocol)

	return model.QueryResult{
		Unit:        unit,
		Domain:      domain,
		Host:        host,
		Protocol:    protocol,
		URL:         url,
		IP:          ip,
		Port:        finalPort,
		StatusCode:  statusCode,
		Length:      length,
		Title:       title,
		Source:      "hunter",
		Reliability: 0,
	}
}

// constructHunterURL 根据Hunter数据构造完整URL
func constructHunterURL(host, ip string, port int, protocol string) string {
	if host == "" && ip == "" {
		return ""
	}

	// 确定使用host还是ip
	targetHost := host
	if targetHost == "" {
		targetHost = ip
	}

	// 处理协议
	if protocol == "" {
		// 根据端口推断协议
		if port == 80 {
			protocol = "http"
		} else if port == 443 {
			protocol = "https"
		} else {
			protocol = "http" // 默认使用http
		}
	}

	// 特殊处理：http + 443端口自动转为https
	if protocol == "http" && port == 443 {
		protocol = "https"
	}

	// 构造URL
	if port > 0 {
		// http 80 和 https 443 不添加端口
		if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
			return protocol + "://" + targetHost
		} else {
			return protocol + "://" + targetHost + ":" + strconv.Itoa(port)
		}
	}

	return protocol + "://" + targetHost
}

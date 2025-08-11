package query

import (
	"bytes"
	"cyberspace_mapping_summary/internal/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// QuakeConfig 用于配置API Key
type QuakeConfig struct {
	APIKey string
}

// QuakeAPIResponse 定义API返回结构
type QuakeAPIResponse struct {
	Meta struct {
		PaginationID string `json:"pagination_id"`
	} `json:"meta"`
	Data []map[string]interface{} `json:"data"`
}

// buildInitialPayload 构造初始查询体
func buildInitialPayload(target string) map[string]interface{} {
	// 判断查询类型
	isCIDR := isCIDR(target)
	isIP := isIP(target)

	queryField := "domain"
	if isIP || isCIDR {
		queryField = "ip"
	}

	return map[string]interface{}{
		"query":        fmt.Sprintf(`%s:"%s"`, queryField, target),
		"start":        0,
		"size":         1000,
		"ignore_cache": true,
		"latest":       true,
	}
}

// buildPaginationPayload 构造翻页查询体
func buildPaginationPayload(target, paginationID string) map[string]interface{} {
	// 判断查询类型
	isCIDR := isCIDR(target)
	isIP := isIP(target)

	queryField := "domain"
	if isIP || isCIDR {
		queryField = "ip"
	}

	return map[string]interface{}{
		"query":         fmt.Sprintf(`%s:"%s"`, queryField, target),
		"pagination_id": paginationID,
		"size":          1000,
		"ignore_cache":  true,
		"latest":        true,
	}
}

// QueryQuake 单域名或IP查询接口，返回结果列表或错误
func QueryQuake(target, apiKey string) ([]model.QueryResult, error) {
	// 构造查询语法用于日志显示
	var querySyntax string
	if isCIDR(target) || isIP(target) {
		querySyntax = fmt.Sprintf(`ip:"%s"`, target)
	} else {
		querySyntax = fmt.Sprintf(`domain:"%s"`, target)
	}

	fmt.Printf("[Quake] 开始扫描: %s (查询语法: %s)\n", target, querySyntax)

	// 使用重试机制执行查询
	return retryWithBackoff("Quake", target, func() ([]model.QueryResult, error) {
		return queryQuakeInternal(target, apiKey)
	})
}

// queryQuakeInternal Quake查询的内部实现
func queryQuakeInternal(target, apiKey string) ([]model.QueryResult, error) {
	var allResults []model.QueryResult
	client := &http.Client{Timeout: 30 * time.Second}

	payload := buildInitialPayload(target)
	paginationID := ""
	page := 0

	for {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("json marshal failed: %w", err)
		}

		req, err := http.NewRequest("POST", "https://quake.360.net/api/v3/scroll/quake_service", bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("request creation failed: %w", err)
		}

		req.Header.Set("X-QuakeToken", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response failed: %w", err)
		}

		var quakeResp QuakeAPIResponse
		if err := json.Unmarshal(respBody, &quakeResp); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}

		pageResults := 0
		for _, item := range quakeResp.Data {
			result := convertQuakeItemToResult("", item)
			allResults = append(allResults, result)
			pageResults++
		}

		fmt.Printf("[Quake] 第%d页扫描完成: %s -> 获得%d条结果\n", page+1, target, pageResults)

		if len(quakeResp.Data) < 1000 {
			break
		}

		paginationID = quakeResp.Meta.PaginationID
		payload = buildPaginationPayload(target, paginationID)
		page++
	}

	fmt.Printf("[Quake] 扫描完成: %s -> 总计%d条结果\n", target, len(allResults))
	return allResults, nil
}

// convertQuakeItemToResult 转换单条结果为模型结构
func convertQuakeItemToResult(unit string, item map[string]interface{}) model.QueryResult {
	// 1. Domain/Host 分离处理
	domain := ""
	host := ""

	// 获取原始domain信息
	if v, ok := item["domain"].(string); ok && v != "" {
		// 判断是否为IP地址，如果是IP则domain留空，否则domain=host
		if isIPOrCIDR(v) {
			domain = "" // IP地址时domain留空
		} else {
			domain = v // 域名时domain=host
		}
	}

	// 2. IP
	ip := ""
	if v, ok := item["ip"].(string); ok {
		ip = v
	}

	// 3. Port
	port := intFromAny(item["port"])

	// 4. HTTP 相关 - 从 service.http 路径获取
	url := ""
	statusCode := 0
	length := 0
	title := ""
	protocol := ""
	protocolWithHost := ""

	// 构造URL并获取协议、host
	url, protocolWithHost = constructURLFromItem(item) // 返回URL、协议和host的拼接组合
	protocol = strings.Split(protocolWithHost, ";")[0]
	host = strings.Split(protocolWithHost, ";")[1]

	// web要从 service.http 路径获取其他信息
	if protocol == "http" || protocol == "https" {
		if serviceMap, ok := item["service"].(map[string]interface{}); ok {
			if httpMap, ok := serviceMap["http"].(map[string]interface{}); ok {
				// status_code
				statusCode = intFromAny(httpMap["status_code"])
				// title - 清理两端的换行符和空格
				if t, ok := httpMap["title"].(string); ok {
					title = strings.TrimSpace(strings.Trim(t, "\n"))
				}
				// body length
				if body, ok := httpMap["body"].(string); ok {
					length = len(body)
				}
			}
		}
	} else {
		// 非web，url为协议+host+端口，防止出现一堆空结果互相“去重”后抵消，导致结果不全
		url = protocol + "://" + host + ":" + strconv.Itoa(port)
	}

	return model.QueryResult{
		Unit:        unit,
		Domain:      domain,
		Host:        host,
		Protocol:    protocol,
		URL:         url,
		IP:          ip,
		Port:        port,
		StatusCode:  statusCode,
		Length:      length,
		Title:       title,
		Source:      "quake",
		Reliability: 0,
	}
}

// constructURLFromItem 根据item信息构造完整URL
func constructURLFromItem(item map[string]interface{}) (string, string) {
	// 1. 获取协议
	protocol := ""
	if serviceMap, ok := item["service"].(map[string]interface{}); ok {
		if serviceName, ok := serviceMap["name"].(string); ok {
			switch serviceName {
			case "http/ssl", "https":
				protocol = "https"
			case "http":
				protocol = "http"
			default:
				protocol = serviceName
			}
		}
	}

	// 2. 获取ip、端口
	ip := item["ip"].(string)
	port := intFromAny(item["port"])

	// 3.分类讨论是web还是其他协议
	host := ""
	baseURL := ""
	// (1)如果是web，去service->http取数据
	if protocol == "http" || protocol == "https" {
		// 获取host
		if serviceMap, ok := item["service"].(map[string]interface{}); ok {
			if httpMap, ok := serviceMap["http"].(map[string]interface{}); ok {
				if h, ok := httpMap["host"].(string); ok && h != "" {
					host = h
				}
			}
		}
		// 如果从service.http中获取host失败，则根据domain情况设置host
		if host == "" {
			if domain, ok := item["domain"].(string); ok && domain != "" {
				host = domain
			} else {
				host = ip
			}
		}
		// 4. 构造基础URL-web
		if port > 0 && (protocol == "http" || protocol == "https") {
			// http 80 和 https 443 不添加端口
			if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
				baseURL = protocol + "://" + host
			} else if protocol == "http" && port == 443 { //http 443 自动转为https，不加端口
				baseURL = "https://" + host
			} else if protocol == "https" && port == 80 { //https 80 自动转为http，不加端口
				baseURL = "http://" + host
			} else {
				baseURL = protocol + "://" + host + ":" + strconv.Itoa(port) //其他情况 protocol+host+port
			}
		}
		// 5. 获取并添加路径
		if serviceMap, ok := item["service"].(map[string]interface{}); ok {
			if httpMap, ok := serviceMap["http"].(map[string]interface{}); ok {
				// quake中单独拥有的path字段要添加到baseURL中
				if path, ok := httpMap["path"].(string); ok && path != "" && path != "/" {
					// 确保路径以/开头
					if !strings.HasPrefix(path, "/") {
						path = "/" + path
					}
					baseURL += path
				}
			}
		}
	} else {
		// 非web，尝试使用item中的hostname、domain、ip
		if v, ok := item["hostname"].(string); ok && v != "" {
			host = v
		} else if v, ok := item["domain"].(string); ok && v != "" {
			host = v
		} else if v, ok := item["ip"].(string); ok && v != "" {
			host = v
		}
		// 非web，url为空，不修改
	}
	// 返回URL、协议和host的拼接组合
	protocolWithHost := protocol + ";" + host
	return baseURL, protocolWithHost
}

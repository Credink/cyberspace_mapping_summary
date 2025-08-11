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

// FofaConfig 用于配置API Key
type FofaConfig struct {
	APIKey string
}

// FofaAPIResponse 定义API返回结构
type FofaAPIResponse struct {
	Error           bool                     `json:"error"`
	ConsumedFpoint  int                      `json:"consumed_fpoint"`
	RequiredFpoints int                      `json:"required_fpoints"`
	Size            int                      `json:"size"`
	Page            int                      `json:"page"`
	Mode            string                   `json:"mode"`
	Query           string                   `json:"query"`
	Results         []map[string]interface{} `json:"results"` // results是对象数组
}

// buildFofaQuery 构造FOFA查询参数
func buildFofaQuery(target, apiKey string, page, size int, fields string) url.Values {
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

	// Base64编码查询语法
	queryBase64 := base64.StdEncoding.EncodeToString([]byte(query))

	params := url.Values{}
	params.Set("qbase64", queryBase64)
	params.Set("fields", fields)
	params.Set("page", strconv.Itoa(page))
	params.Set("size", strconv.Itoa(size))
	params.Set("full", "false")
	params.Set("r_type", "json")
	params.Set("key", apiKey)

	return params
}

// QueryFofa 单域名或IP查询接口，返回结果列表或错误
func QueryFofa(target, apiKey string) ([]model.QueryResult, error) {
	// 构造查询语法用于日志显示
	var querySyntax string
	if isCIDR(target) {
		querySyntax = fmt.Sprintf(`ip="%s"`, target)
	} else if isIP(target) {
		querySyntax = fmt.Sprintf(`ip="%s"`, target)
	} else {
		querySyntax = fmt.Sprintf(`domain="%s"`, target)
	}

	fmt.Printf("[FOFA] 开始扫描: %s (查询语法: %s)\n", target, querySyntax)

	// 使用重试机制执行查询
	return retryWithBackoff("FOFA", target, func() ([]model.QueryResult, error) {
		return queryFofaInternal(target, apiKey)
	})
}

// queryFofaInternal FOFA查询的内部实现
func queryFofaInternal(target, apiKey string) ([]model.QueryResult, error) {
	var allResults []model.QueryResult
	client := &http.Client{Timeout: 30 * time.Second}

	// 默认字段：host,ip,port,protocol,title,server,domain
	fields := "host,ip,port,protocol,title,server,domain"
	page := 1
	size := 1000 // FOFA默认每页1000条

	for {
		params := buildFofaQuery(target, apiKey, page, size, fields)

		// 构造请求URL
		baseURL := "https://fofa.info/api/v1/search/all"
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

		var fofaResp FofaAPIResponse
		if err := json.Unmarshal(respBody, &fofaResp); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}

		// 检查API错误
		if fofaResp.Error {
			return nil, fmt.Errorf("FOFA API error: %s", string(respBody))
		}

		// 转换结果
		pageResults := 0
		for _, result := range fofaResp.Results {
			queryResult := convertFofaItemToResult("", result)
			allResults = append(allResults, queryResult)
			pageResults++
		}

		fmt.Printf("[FOFA] 第%d页扫描完成: %s -> 获得%d条结果\n", page, target, pageResults)

		// 检查是否还有更多数据
		if len(fofaResp.Results) < size {
			break
		}

		page++

		// 防止无限循环
		if page > 100 {
			break
		}
	}

	fmt.Printf("[FOFA] 扫描完成: %s -> 总计%d条结果\n", target, len(allResults))
	return allResults, nil
}

// convertFofaItemToResult 转换FOFA单条结果为模型结构
func convertFofaItemToResult(unit string, item map[string]interface{}) model.QueryResult {
	// 从map中提取字段
	rawHost := stringFromAny(item["host"])
	ip := stringFromAny(item["ip"])
	port := intFromAny(item["port"])
	protocol := stringFromAny(item["protocol"])
	title := strings.TrimSpace(strings.Trim(stringFromAny(item["title"]), "\n"))
	domain := stringFromAny(item["domain"])

	// 处理host字段：提取域名部分，但保留端口号信息
	host, hostPort := extractHostAndPort(rawHost)

	// 兜底策略：如果host为空，根据domain情况设置host
	if host == "" {
		if domain != "" {
			host = domain
		} else {
			host = ip
		}
	}

	// 处理domain字段
	// 如果domain为空且host不是IP，则domain=host
	if domain == "" && host != "" && !isIPOrCIDR(host) {
		domain = host
	}

	// 构造URL：如果host中已有端口号，使用host中的端口；否则使用API返回的端口
	finalPort := port
	if hostPort > 0 {
		finalPort = hostPort
	}
	url := constructFofaURL(host, ip, finalPort, protocol)

	// 获取HTTP状态码和长度（FOFA API不直接提供，设为0）
	statusCode := 0
	length := 0

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
		Source:      "fofa",
		Reliability: 0,
	}
}

// constructFofaURL 根据FOFA数据构造完整URL
func constructFofaURL(host, ip string, port int, protocol string) string {
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

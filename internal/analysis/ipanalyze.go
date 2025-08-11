package analysis

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// IPAnalysisResult 表示IP分析结果
type IPAnalysisResult struct {
	IP       string
	URLCount int
	Domains  []string
	Sources  []string
}

// AnalyzeIPBusinessCount 分析IP业务数量
func AnalyzeIPBusinessCount(db *sql.DB, tableName string, threshold int) ([]IPAnalysisResult, error) {
	log.Printf("[*] 开始IP业务数量分析，阈值: %d", threshold)

	// 查询每个IP关联的URL数量
	query := fmt.Sprintf(`
		SELECT ip, COUNT(DISTINCT url) as url_count, 
		       GROUP_CONCAT(DISTINCT domain) as domains,
		       GROUP_CONCAT(DISTINCT source) as sources
		FROM %s 
		WHERE ip IS NOT NULL AND ip != ''
		GROUP BY ip 
		HAVING url_count >= ?
		ORDER BY url_count DESC
	`, tableName)

	rows, err := db.Query(query, threshold)
	if err != nil {
		return nil, fmt.Errorf("查询IP业务数量失败: %v", err)
	}
	defer rows.Close()

	var results []IPAnalysisResult
	for rows.Next() {
		var ip, domainsStr, sourcesStr string
		var urlCount int

		if err := rows.Scan(&ip, &urlCount, &domainsStr, &sourcesStr); err != nil {
			continue
		}

		// 解析域名和来源
		domains := strings.Split(domainsStr, ",")
		sources := strings.Split(sourcesStr, ",")

		// 去重
		domains = removeDuplicates(domains)
		sources = removeDuplicates(sources)

		results = append(results, IPAnalysisResult{
			IP:       ip,
			URLCount: urlCount,
			Domains:  domains,
			Sources:  sources,
		})
	}

	log.Printf("[*] 发现 %d 个高业务量IP", len(results))
	return results, nil
}

// ExportIPAnalysisResults 导出IP分析结果
func ExportIPAnalysisResults(results []IPAnalysisResult, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入UTF-8 BOM
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	// 创建CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入CSV头部
	header := []string{"IP地址", "URL数量", "关联域名", "数据来源", "生成时间"}
	err = writer.Write(header)
	if err != nil {
		return fmt.Errorf("写入CSV头部失败: %v", err)
	}

	// 写入数据行
	currentTime := getCurrentTime()
	for _, result := range results {
		row := []string{
			result.IP,
			fmt.Sprintf("%d", result.URLCount),
			strings.Join(result.Domains, ";"),
			strings.Join(result.Sources, ";"),
			currentTime,
		}
		err = writer.Write(row)
		if err != nil {
			return fmt.Errorf("写入CSV数据行失败: %v", err)
		}
	}

	log.Printf("[*] IP分析结果已导出到: %s", outputPath)
	return nil
}

// removeDuplicates 去除字符串切片中的重复项
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// getCurrentTime 获取当前时间字符串
func getCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

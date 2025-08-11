package database

import (
	"cyberspace_mapping_summary/internal/model"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// 统一 URL 去重格式处理
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "/")
	if strings.HasPrefix(raw, "http://") && strings.HasSuffix(raw, ":80") {
		raw = strings.TrimSuffix(raw, ":80")
	}
	if strings.HasPrefix(raw, "https://") && strings.HasSuffix(raw, ":443") {
		raw = strings.TrimSuffix(raw, ":443")
	}
	return raw
}

// 合并字符串（去重 + 分号连接）
func mergeValues(a, b string) string {
	m := make(map[string]bool)
	for _, val := range strings.Split(a+";"+b, ";") {
		val = strings.TrimSpace(val)
		if val != "" {
			m[val] = true
		}
	}
	var result []string
	for k := range m {
		result = append(result, k)
	}
	return strings.Join(result, ";")
}

// InitDB 初始化 SQLite 数据库和数据表
func InitDB(dbPath string, tableName string) (*sql.DB, error) {
	os.MkdirAll(filepath.Dir(dbPath), os.ModePerm)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 设置数据库编码为UTF-8
	_, err = db.Exec("PRAGMA encoding = 'UTF-8'")
	if err != nil {
		return nil, err
	}

	createStmt := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    org_code TEXT,
    domain TEXT,
    host TEXT,
    protocol TEXT,
    url TEXT UNIQUE,
    ip TEXT,
    port INTEGER,
    status_code INTEGER,
    length INTEGER,
    title TEXT,
    source TEXT,
    reliability INTEGER
);
`, tableName)

	_, err = db.Exec(createStmt)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// SaveResults 去重并写入数据库
func SaveResults(db *sql.DB, tableName string, results []model.QueryResult) error {
	// 先查询现有数据，用于去重处理
	querySQL := fmt.Sprintf("SELECT title, source, reliability FROM %s WHERE url = ?", tableName)

	insertSQL := fmt.Sprintf(`
INSERT INTO %s (org_code, domain, host, protocol, url, ip, port, status_code, length, title, source, reliability)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(url) DO UPDATE SET
    title=?,
    source=?,
    reliability=?;
`, tableName)

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	queryStmt, err := tx.Prepare(querySQL)
	if err != nil {
		return err
	}
	defer queryStmt.Close()

	insertStmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	for _, r := range results {
		normURL := NormalizeURL(r.URL)

		// 查询现有数据
		var existingTitle, existingSource string
		var existingReliability int
		err := queryStmt.QueryRow(normURL).Scan(&existingTitle, &existingSource, &existingReliability)

		if err == sql.ErrNoRows {
			// 新记录，直接插入
			_, err := insertStmt.Exec(
				r.Unit,
				r.Domain,
				r.Host,
				r.Protocol,
				normURL,
				r.IP,
				r.Port,
				r.StatusCode,
				r.Length,
				r.Title,
				r.Source,
				r.Reliability,
				r.Title, // 用于ON CONFLICT
				r.Source,
				r.Reliability,
			)
			if err != nil {
				log.Printf("insert error for URL %s: %v", normURL, err)
				continue
			}
		} else if err != nil {
			log.Printf("query error for URL %s: %v", normURL, err)
			continue
		} else {
			// 记录已存在，合并数据
			mergedTitle := mergeValues(existingTitle, r.Title)
			mergedSource := mergeValues(existingSource, r.Source)
			mergedReliability := existingReliability

			// 根据reliability规则设置：Reliability 0是最高可信度，不能被覆盖
			// 只有当新记录的reliability更低时，才更新为更低的reliability
			if existingReliability == 0 {
				// 如果现有记录是0（最高可信度），保持不变
				mergedReliability = 0
			} else if r.Reliability == 0 {
				// 如果新记录是0（最高可信度），更新为0
				mergedReliability = 0
			} else if r.Reliability < existingReliability {
				// 如果新记录的reliability更低，更新为更低的reliability
				mergedReliability = r.Reliability
			} else {
				// 否则保持现有的reliability
				mergedReliability = existingReliability
			}

			_, err := insertStmt.Exec(
				r.Unit,
				r.Domain,
				r.Host,
				r.Protocol,
				normURL,
				r.IP,
				r.Port,
				r.StatusCode,
				r.Length,
				r.Title,
				r.Source,
				r.Reliability,
				mergedTitle, // 用于ON CONFLICT
				mergedSource,
				mergedReliability, // 使用合并后的reliability
			)
			if err != nil {
				log.Printf("update error for URL %s: %v", normURL, err)
				continue
			}
		}
	}

	return tx.Commit()
}

// IsIPExists 检查指定IP是否在数据库中已存在
func IsIPExists(db *sql.DB, tableName, ip string) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE ip = ?", tableName)
	var count int
	err := db.QueryRow(query, ip).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetExistingIPs 获取数据库中已存在的所有IP
func GetExistingIPs(db *sql.DB, tableName string) (map[string]bool, error) {
	query := fmt.Sprintf("SELECT DISTINCT ip FROM %s WHERE ip IS NOT NULL AND ip != ''", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existingIPs := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		existingIPs[ip] = true
	}

	return existingIPs, nil
}

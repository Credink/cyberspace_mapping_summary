package exporter

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
)

func ExportTableToCSV(db *sql.DB, tableName, outputPath string) error {
	query := fmt.Sprintf("SELECT org_code, domain, host, protocol, url, ip, port, status_code, length, title, source, reliability FROM %s", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入UTF-8 BOM，确保Excel等软件能正确识别中文
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write([]string{
		"OrgCode", "Domain", "Host", "Protocol", "URL", "IP", "Port", "StatusCode", "Length", "Title", "Source", "Reliability",
	})

	for rows.Next() {
		var org, domain, host, protocol, url, ip, title, source string
		var port, status, length, reliability int

		err := rows.Scan(&org, &domain, &host, &protocol, &url, &ip, &port, &status, &length, &title, &source, &reliability)
		if err != nil {
			return err
		}

		record := []string{
			org,
			domain,
			host,
			protocol,
			url,
			ip,
			fmt.Sprintf("%d", port),
			fmt.Sprintf("%d", status),
			fmt.Sprintf("%d", length),
			title,
			source,
			fmt.Sprintf("%d", reliability),
		}
		writer.Write(record)
	}

	return nil
}

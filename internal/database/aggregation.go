package database

import (
	"database/sql"
	"fmt"
	"net"
	"sort"
	"strings"
)

func GetHighDensityCIDRs(db *sql.DB, tableName string, threshold int) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT ip FROM %s", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cSegmentCount := make(map[string]int)

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			continue // skip non-IPv4
		}

		ipParts := strings.Split(ip, ".")
		if len(ipParts) != 4 {
			continue
		}

		cidr := fmt.Sprintf("%s.%s.%s.0/24", ipParts[0], ipParts[1], ipParts[2])
		cSegmentCount[cidr]++
	}

	var result []string
	for cidr, count := range cSegmentCount {
		if count >= threshold {
			result = append(result, cidr)
		}
	}

	sort.Strings(result)
	return result, nil
}

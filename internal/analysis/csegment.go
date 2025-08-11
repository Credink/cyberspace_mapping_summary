package analysis

import (
	"cyberspace_mapping_summary/internal/database"
	"cyberspace_mapping_summary/internal/model"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// CSegmentInfo 表示C段信息
type CSegmentInfo struct {
	CIDR          string
	Organizations []string
	IsMixed       bool
}

// CSegmentAnalysis 执行C段分析
func CSegmentAnalysis(db *sql.DB, tableName string, threshold int) ([]CSegmentInfo, error) {
	log.Printf("[*] 开始C段分析，阈值: %d", threshold)

	// 获取高密度C段
	cidrs, err := database.GetHighDensityCIDRs(db, tableName, threshold)
	if err != nil {
		return nil, fmt.Errorf("获取高密度C段失败: %v", err)
	}

	var cSegmentInfos []CSegmentInfo
	log.Printf("[*] 发现 %d 个高密度C段", len(cidrs))

	for i, cidr := range cidrs {
		// 分析C段的组织归属
		organizations, err := getOrganizationsInCIDR(db, tableName, cidr)
		if err != nil {
			log.Printf("[!] 分析C段 %s 组织归属失败: %v", cidr, err)
			continue
		}

		// 判断是否为混合C段（包含多个组织）
		isMixed := len(organizations) > 1

		cSegmentInfo := CSegmentInfo{
			CIDR:          cidr,
			Organizations: organizations,
			IsMixed:       isMixed,
		}

		cSegmentInfos = append(cSegmentInfos, cSegmentInfo)

		log.Printf("[%d] %s (组织: %s, 混合: %t)",
			i+1, cidr, strings.Join(organizations, ";"), isMixed)
	}

	return cSegmentInfos, nil
}

// getOrganizationsInCIDR 获取指定C段中的组织列表
func getOrganizationsInCIDR(db *sql.DB, tableName, cidr string) ([]string, error) {
	// 提取C段前缀
	ipParts := strings.Split(cidr, "/")
	if len(ipParts) != 2 {
		return nil, fmt.Errorf("无效的CIDR格式: %s", cidr)
	}

	baseIP := ipParts[0]
	prefix := strings.TrimSuffix(baseIP, ".0") + "."

	// 查询该C段中所有IP对应的组织
	query := fmt.Sprintf("SELECT DISTINCT org_code FROM %s WHERE ip LIKE ? AND org_code IS NOT NULL AND org_code != ''", tableName)
	rows, err := db.Query(query, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var organizations []string
	for rows.Next() {
		var org string
		if err := rows.Scan(&org); err != nil {
			continue
		}
		organizations = append(organizations, org)
	}

	return organizations, nil
}

// GenerateSecondRoundTargets 根据C段生成第二轮查询目标，过滤掉第一轮已查询的C段
func GenerateSecondRoundTargets(cSegmentInfos []CSegmentInfo, firstRoundTargets []model.TargetEntry) []model.TargetEntry {
	var targets []model.TargetEntry

	// 创建第一轮查询目标的C段映射，用于快速查找
	firstRoundCIDRs := make(map[string]bool)
	for _, target := range firstRoundTargets {
		// 检查目标是否为C段格式
		if isCIDRFormat(target.Host) {
			firstRoundCIDRs[target.Host] = true
		}
	}

	for _, cSegmentInfo := range cSegmentInfos {
		// 检查该C段是否已在第一轮中查询过
		if firstRoundCIDRs[cSegmentInfo.CIDR] {
			fmt.Printf("[*] 跳过已在第一轮查询的C段: %s\n", cSegmentInfo.CIDR)
			continue
		}

		// 根据组织归属设置单位名称
		var unitName string
		if cSegmentInfo.IsMixed {
			unitName = "混合C段"
		} else if len(cSegmentInfo.Organizations) == 1 {
			unitName = cSegmentInfo.Organizations[0]
		} else {
			unitName = "未知组织"
		}

		targets = append(targets, model.TargetEntry{
			Unit: unitName,
			Host: cSegmentInfo.CIDR, // 直接使用CIDR格式
		})
	}

	return targets
}

// isCIDRFormat 检查字符串是否为CIDR格式
func isCIDRFormat(s string) bool {
	// 简单的CIDR格式检查：包含"/"且以数字结尾
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}

	// 检查前缀是否为有效IP
	ip := parts[0]
	ipParts := strings.Split(ip, ".")
	if len(ipParts) != 4 {
		return false
	}

	// 检查后缀是否为有效数字
	if parts[1] == "24" || parts[1] == "16" || parts[1] == "8" {
		return true
	}

	return false
}

// GetExistingIPsInCIDR 获取指定C段中已存在的IP
func GetExistingIPsInCIDR(db *sql.DB, tableName, cidr string) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT ip FROM %s WHERE ip LIKE ?", tableName)

	// 提取C段前缀
	ipParts := strings.Split(cidr, "/")
	if len(ipParts) != 2 {
		return nil, fmt.Errorf("无效的CIDR格式: %s", cidr)
	}

	baseIP := ipParts[0]
	prefix := strings.TrimSuffix(baseIP, ".0") + "."

	rows, err := db.Query(query, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		ips = append(ips, ip)
	}

	return ips, nil
}

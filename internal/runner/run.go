package runner

import (
	"cyberspace_mapping_summary/internal/config"
	"cyberspace_mapping_summary/internal/database"
	"cyberspace_mapping_summary/internal/exporter"
	"cyberspace_mapping_summary/internal/loader"
	"cyberspace_mapping_summary/internal/model"
	"cyberspace_mapping_summary/internal/query"
	"cyberspace_mapping_summary/internal/util"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func RunAll(cfg *config.Config) error {
	// 时间戳处理
	taskID := util.GenerateTaskID()
	projectDir := util.GenerateProjectDir(cfg.Output.BaseDir)
	projectPath := filepath.Join(cfg.Output.BaseDir, projectDir)

	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return err
	}

	// 读取并验证目标
	targets, err := loader.ReadTargetsFromCSV(cfg.Input.TargetFile)
	if err != nil {
		return fmt.Errorf("读取 CSV 失败: %v", err)
	}

	var validTargets []model.TargetEntry
	for _, t := range targets {
		if util.IsValidHost(t.Host) {
			validTargets = append(validTargets, t)
		}
	}

	if len(validTargets) == 0 {
		return fmt.Errorf("没有有效的目标")
	}

	// 初始化数据库
	dbPath := filepath.Join(projectPath, "res.db")
	tableName := util.GenerateTableName(taskID)
	sqliteDB, err := database.InitDB(dbPath, tableName)
	if err != nil {
		return err
	}

	//if err := database.CreateResultTable(sqliteDB, tableName); err != nil {
	//	return err
	//}

	// 遍历空间测绘 API，逐个查询并入库（reliability = 0）
	if cfg.APIKeys.Quake != "" {
		for _, target := range validTargets {
			quakeRes, err := query.QueryQuake(target.Host, cfg.APIKeys.Quake)
			if err != nil {
				log.Printf("[quake] 查询失败: %s", target.Host)
				continue
			}
			database.SaveResults(sqliteDB, tableName, quakeRes)
		}
	}

	// FOFA查询
	if cfg.APIKeys.FOFA != "" {
		for _, target := range validTargets {
			fofaRes, err := query.QueryFofa(target.Host, cfg.APIKeys.FOFA)
			if err != nil {
				log.Printf("[fofa] 查询失败: %s", target.Host)
				continue
			}
			database.SaveResults(sqliteDB, tableName, fofaRes)
		}
	}

	// Hunter查询
	if cfg.APIKeys.Hunter != "" {
		for _, target := range validTargets {
			hunterRes, err := query.QueryHunter(target.Host, cfg.APIKeys.Hunter)
			if err != nil {
				log.Printf("[hunter] 查询失败: %s", target.Host)
				continue
			}
			database.SaveResults(sqliteDB, tableName, hunterRes)
		}
	}

	// 导出第一轮结果
	csvFileName := util.GenerateCSVFileName(taskID, "domain_step1")
	exportPath := filepath.Join(projectPath, csvFileName)
	if err := exporter.ExportTableToCSV(sqliteDB, tableName, exportPath); err != nil {
		return fmt.Errorf("导出CSV失败: %v", err)
	}

	return nil
}

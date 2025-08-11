package main

import (
	"cyberspace_mapping_summary/internal/analysis"
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
	"sync"
	"time"
)

func main() {
	// 1. 读取配置
	cfg, shouldExit, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	if shouldExit {
		fmt.Println("程序已退出，请配置好相关文件后重新运行。")
		os.Exit(0)
	}

	// 定义查询间隔时间（秒）
	queryInterval := time.Duration(cfg.Query.IntervalSeconds) * time.Second

	// 2. 生成任务ID和结果目录
	taskID := util.GenerateTaskID()
	projectDir := util.GenerateProjectDir(cfg.Output.BaseDir)
	resultsDir := filepath.Join("results", projectDir)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		log.Fatalf("创建结果目录失败: %v", err)
	}
	fmt.Printf("[+] 项目结果将保存在: %s\n", resultsDir)

	// 3. 读取目标（csv，带单位）
	targetFile := "targets.csv"
	if cfg.Input.TargetFile != "" {
		targetFile = cfg.Input.TargetFile
	}
	targets, err := loader.ReadTargetsFromCSV(targetFile)
	if err != nil {
		log.Fatalf("读取目标文件失败: %v", err)
	}
	fmt.Printf("[*] 共加载目标: %d 条\n", len(targets))

	// 4. 验证域名/IP有效性
	validTargets := make([]model.TargetEntry, 0)
	for _, t := range targets {
		if util.IsValidHost(t.Host) {
			validTargets = append(validTargets, t)
		}
	}
	fmt.Printf("[*] 有效目标: %d 条\n", len(validTargets))
	if len(validTargets) == 0 {
		log.Fatalf("无有效目标，退出")
	}

	// 5. 并发查询三大测绘平台
	allResults := make([]model.QueryResult, 0)
	var allResultsMutex sync.Mutex
	var wg sync.WaitGroup

	// 检查API Key配置
	if cfg.APIKeys.Quake == "" && cfg.APIKeys.FOFA == "" && cfg.APIKeys.Hunter == "" {
		log.Fatalf("未配置任何API Key，无法进行查询")
	}

	// Quake查询协程
	if cfg.APIKeys.Quake != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println("[*] 开始Quake查询...")
			quakeResults := make([]model.QueryResult, 0)

			for _, t := range validTargets {
				results, err := query.QueryQuake(t.Host, cfg.APIKeys.Quake)
				if err != nil {
					log.Printf("[!] Quake查询 %s 失败: %v", t.Host, err)
					continue
				}
				// 补充单位归属
				for i := range results {
					results[i].Unit = t.Unit
				}
				quakeResults = append(quakeResults, results...)

				// 每轮查询后sleep
				time.Sleep(queryInterval)
			}

			// 线程安全地添加结果
			allResultsMutex.Lock()
			allResults = append(allResults, quakeResults...)
			allResultsMutex.Unlock()

			fmt.Printf("[*] Quake查询完成，结果数: %d 条\n", len(quakeResults))
		}()
	} else {
		fmt.Println("[*] 未配置Quake API Key，跳过Quake查询")
	}

	// FOFA查询协程
	if cfg.APIKeys.FOFA != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println("[*] 开始FOFA查询...")
			fofaResults := make([]model.QueryResult, 0)

			for _, t := range validTargets {
				results, err := query.QueryFofa(t.Host, cfg.APIKeys.FOFA)
				if err != nil {
					log.Printf("[!] FOFA查询 %s 失败: %v", t.Host, err)
					continue
				}
				// 补充单位归属
				for i := range results {
					results[i].Unit = t.Unit
				}
				fofaResults = append(fofaResults, results...)

				// 每轮查询后sleep
				time.Sleep(queryInterval)
			}

			// 线程安全地添加结果
			allResultsMutex.Lock()
			allResults = append(allResults, fofaResults...)
			allResultsMutex.Unlock()

			fmt.Printf("[*] FOFA查询完成，结果数: %d 条\n", len(fofaResults))
		}()
	} else {
		fmt.Println("[*] 未配置FOFA API Key，跳过FOFA查询")
	}

	// Hunter查询协程
	if cfg.APIKeys.Hunter != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println("[*] 开始Hunter查询...")
			hunterResults := make([]model.QueryResult, 0)

			for _, t := range validTargets {
				results, err := query.QueryHunter(t.Host, cfg.APIKeys.Hunter)
				if err != nil {
					log.Printf("[!] Hunter查询 %s 失败: %v", t.Host, err)
					continue
				}
				// 补充单位归属
				for i := range results {
					results[i].Unit = t.Unit
				}
				hunterResults = append(hunterResults, results...)

				// 每轮查询后sleep
				time.Sleep(queryInterval)
			}

			// 线程安全地添加结果
			allResultsMutex.Lock()
			allResults = append(allResults, hunterResults...)
			allResultsMutex.Unlock()

			fmt.Printf("[*] Hunter查询完成，结果数: %d 条\n", len(hunterResults))
		}()
	} else {
		fmt.Println("[*] 未配置Hunter API Key，跳过Hunter查询")
	}

	// 等待所有协程完成
	fmt.Println("[*] 等待所有查询完成...")
	wg.Wait()
	fmt.Printf("[*] 所有查询完成，总结果数: %d 条\n", len(allResults))

	// 8. 初始化数据库和表
	dbPath := "res.db"
	tableName := util.GenerateTableName(taskID)
	db, err := database.InitDB(dbPath, tableName)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()

	// 9. 去重并保存到sqlite
	err = database.SaveResults(db, tableName, allResults)
	if err != nil {
		log.Fatalf("数据保存失败: %v", err)
	}
	fmt.Println("[*] 数据已保存到sqlite")

	// 查询去重后的数据数量
	var count int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err = db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		log.Printf("[!] 查询数据数量失败: %v", err)
	} else {
		fmt.Printf("[*] 去重后数据数量: %d 条\n", count)
	}

	// 10. 导出第一轮结果
	csvFileName := util.GenerateCSVFileName(taskID, "step1")
	outputPath := filepath.Join(resultsDir, csvFileName)
	err = exporter.ExportTableToCSV(db, tableName, outputPath)
	if err != nil {
		log.Fatalf("导出csv失败: %v", err)
	}
	fmt.Println("[*] 已导出第一轮结果到:", outputPath)

	// 11. C段分析与第二轮查询
	if cfg.Query.MinIPsPerCIDR == -1 {
		fmt.Println("[*] 配置为跳过第二轮扫描，跳过C段分析")
	} else {
		fmt.Println("[*] 开始C段分析...")
		cSegmentInfos, err := analysis.CSegmentAnalysis(db, tableName, cfg.Query.MinIPsPerCIDR)
		if err != nil {
			log.Printf("[!] C段分析失败: %v", err)
		} else if len(cSegmentInfos) > 0 {
			fmt.Printf("[*] 发现 %d 个高密度C段，开始第二轮查询\n", len(cSegmentInfos))

			// 生成第二轮查询目标，过滤掉第一轮已查询的C段
			secondRoundTargets := analysis.GenerateSecondRoundTargets(cSegmentInfos, validTargets)
			fmt.Printf("[*] 第二轮查询目标: %d 个\n", len(secondRoundTargets))

			// 获取第一轮查询中已存在的IP列表，用于reliability判断
			existingIPs, err := database.GetExistingIPs(db, tableName)
			if err != nil {
				log.Printf("[!] 获取已存在IP列表失败: %v", err)
				existingIPs = make(map[string]bool) // 使用空map作为fallback
			}
			fmt.Printf("[*] 第一轮查询中已存在 %d 个IP\n", len(existingIPs))

			// 第二轮查询结果
			secondRoundResults := make([]model.QueryResult, 0)
			var secondRoundMutex sync.Mutex
			var secondRoundWg sync.WaitGroup

			// 执行第二轮查询（多协程并发）
			if cfg.APIKeys.Quake != "" {
				secondRoundWg.Add(1)
				go func() {
					defer secondRoundWg.Done()
					fmt.Println("[*] 开始第二轮Quake查询...")
					quakeResults := make([]model.QueryResult, 0)

					for _, t := range secondRoundTargets {
						results, err := query.QueryQuake(t.Host, cfg.APIKeys.Quake)
						if err != nil {
							log.Printf("[!] 第二轮Quake查询 %s 失败: %v", t.Host, err)
							continue
						}
						// 根据IP是否已存在动态设置reliability
						for i := range results {
							results[i].Unit = t.Unit
							// 检查IP是否在第一轮中已存在
							if existingIPs[results[i].IP] {
								results[i].Reliability = 1 // IP已存在，reliability=1
							} else {
								results[i].Reliability = 2 // 新IP，reliability=2
							}
						}
						quakeResults = append(quakeResults, results...)
						time.Sleep(queryInterval)
					}

					// 线程安全地添加结果
					secondRoundMutex.Lock()
					secondRoundResults = append(secondRoundResults, quakeResults...)
					secondRoundMutex.Unlock()

					fmt.Printf("[*] 第二轮Quake查询完成，结果数: %d 条\n", len(quakeResults))
				}()
			}

			if cfg.APIKeys.FOFA != "" {
				secondRoundWg.Add(1)
				go func() {
					defer secondRoundWg.Done()
					fmt.Println("[*] 开始第二轮FOFA查询...")
					fofaResults := make([]model.QueryResult, 0)

					for _, t := range secondRoundTargets {
						results, err := query.QueryFofa(t.Host, cfg.APIKeys.FOFA)
						if err != nil {
							log.Printf("[!] 第二轮FOFA查询 %s 失败: %v", t.Host, err)
							continue
						}
						// 根据IP是否已存在动态设置reliability
						for i := range results {
							results[i].Unit = t.Unit
							// 检查IP是否在第一轮中已存在
							if existingIPs[results[i].IP] {
								results[i].Reliability = 1 // IP已存在，reliability=1
							} else {
								results[i].Reliability = 2 // 新IP，reliability=2
							}
						}
						fofaResults = append(fofaResults, results...)
						time.Sleep(queryInterval)
					}

					// 线程安全地添加结果
					secondRoundMutex.Lock()
					secondRoundResults = append(secondRoundResults, fofaResults...)
					secondRoundMutex.Unlock()

					fmt.Printf("[*] 第二轮FOFA查询完成，结果数: %d 条\n", len(fofaResults))
				}()
			}

			if cfg.APIKeys.Hunter != "" {
				secondRoundWg.Add(1)
				go func() {
					defer secondRoundWg.Done()
					fmt.Println("[*] 开始第二轮Hunter查询...")
					hunterResults := make([]model.QueryResult, 0)

					for _, t := range secondRoundTargets {
						results, err := query.QueryHunter(t.Host, cfg.APIKeys.Hunter)
						if err != nil {
							log.Printf("[!] 第二轮Hunter查询 %s 失败: %v", t.Host, err)
							continue
						}
						// 根据IP是否已存在动态设置reliability
						for i := range results {
							results[i].Unit = t.Unit
							// 检查IP是否在第一轮中已存在
							if existingIPs[results[i].IP] {
								results[i].Reliability = 1 // IP已存在，reliability=1
							} else {
								results[i].Reliability = 2 // 新IP，reliability=2
							}
						}
						hunterResults = append(hunterResults, results...)
						time.Sleep(queryInterval)
					}

					// 线程安全地添加结果
					secondRoundMutex.Lock()
					secondRoundResults = append(secondRoundResults, hunterResults...)
					secondRoundMutex.Unlock()

					fmt.Printf("[*] 第二轮Hunter查询完成，结果数: %d 条\n", len(hunterResults))
				}()
			}

			// 等待所有第二轮查询协程完成
			fmt.Println("[*] 等待所有第二轮查询完成...")
			secondRoundWg.Wait()
			fmt.Printf("[*] 所有第二轮查询完成，总结果数: %d 条\n", len(secondRoundResults))

			// 保存第二轮结果到数据库（自动去重）
			if len(secondRoundResults) > 0 {
				err = database.SaveResults(db, tableName, secondRoundResults)
				if err != nil {
					log.Printf("[!] 保存第二轮结果失败: %v", err)
				} else {
					fmt.Printf("[*] 第二轮查询完成，新增结果: %d 条\n", len(secondRoundResults))
				}
			}

			// 导出第二轮结果
			secondRoundCSV := util.GenerateCSVFileName(taskID, "step2")
			secondRoundPath := filepath.Join(resultsDir, secondRoundCSV)
			err = exporter.ExportTableToCSV(db, tableName, secondRoundPath)
			if err != nil {
				log.Printf("[!] 导出第二轮结果失败: %v", err)
			} else {
				fmt.Println("[*] 已导出第二轮结果到:", secondRoundPath)
			}
		} else {
			fmt.Println("[*] 未发现高密度C段，跳过第二轮查询")
		}
	}

	// 12. IP业务数量分析
	fmt.Println("[*] 开始IP业务数量分析...")
	ipResults, err := analysis.AnalyzeIPBusinessCount(db, tableName, cfg.Query.MinURLsPerIPForFlag)
	if err != nil {
		log.Printf("[!] IP业务数量分析失败: %v", err)
	} else if len(ipResults) > 0 {
		// 导出IP分析结果
		ipAnalysisFile := util.GenerateCSVFileName(taskID, "ip_need_scan")
		ipAnalysisPath := filepath.Join(resultsDir, ipAnalysisFile)
		err = analysis.ExportIPAnalysisResults(ipResults, ipAnalysisPath)
		if err != nil {
			log.Printf("[!] 导出IP分析结果失败: %v", err)
		} else {
			fmt.Printf("[*] 发现 %d 个高业务量IP，已导出到: %s\n", len(ipResults), ipAnalysisPath)
		}
	} else {
		fmt.Println("[*] 未发现高业务量IP")
	}

	fmt.Println("[✔] 主流程执行完毕")
}

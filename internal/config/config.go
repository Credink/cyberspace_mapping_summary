package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKeys struct {
		FOFA   string `yaml:"fofa"`
		Quake  string `yaml:"quake"`
		Hunter string `yaml:"hunter"`
	} `yaml:"api_keys"`

	Query struct {
		MinIPsPerCIDR       int `yaml:"min_ips_per_cidr"`
		MinURLsPerIPForFlag int `yaml:"min_urls_per_ip_for_flag"`
		IntervalSeconds     int `yaml:"interval_seconds"`
	} `yaml:"query"`

	Input struct {
		TargetFile string `yaml:"target_file"`
	} `yaml:"input"`

	Output struct {
		BaseDir string `yaml:"base_dir"`
	} `yaml:"output"`
}

// LoadConfig loads YAML config from file path
// Returns config, shouldExit, error
func LoadConfig(path string) (*Config, bool, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// 如果读取失败，尝试生成默认配置文件
		if os.IsNotExist(err) {
			fmt.Printf("配置文件 %s 不存在，正在生成默认配置文件...\n", path)
			if err := generateDefaultConfig(path); err != nil {
				return nil, true, fmt.Errorf("生成默认配置文件失败: %w", err)
			}
			fmt.Printf("默认配置文件已生成: %s\n", path)
			fmt.Println("请编辑配置文件，填入您的API Key后重新运行程序。")
			fmt.Println("")
			fmt.Println("=== 配置说明 ===")
			fmt.Println("在config.yaml中配置各测绘平台的API Key：")
			fmt.Println("- fofa: FOFA平台的API Key")
			fmt.Println("- quake: Quake平台的API Key")
			fmt.Println("- hunter: Hunter平台的API Key")
			fmt.Println("留空表示不使用该平台")
			fmt.Println("")
			fmt.Println("=== 输出文件说明 ===")
			fmt.Println("程序运行后会在results目录下生成以下文件：")
			fmt.Println("")
			fmt.Println("📁 [时间戳]_step1.csv")
			fmt.Println("   内容：原始扫描结果，包含所有发现的资产信息")
			fmt.Println("   字段：组织代码、域名、主机、协议、URL、IP、端口、状态码、长度、标题、数据来源、可信度")
			fmt.Println("   可信度：0（空间测绘数据，需要进一步验证）")
			fmt.Println("")
			fmt.Println("📁 [时间戳]_ip_need_scan.csv")
			fmt.Println("   内容：需要进一步扫描的IP地址汇总")
			fmt.Println("   字段：IP地址、关联URL数量、关联域名、数据来源、生成时间")
			fmt.Println("   可信度：0（空间测绘数据，需要进一步验证）")
			fmt.Println("")
			fmt.Println("📁 [时间戳]_cider_step2.csv（可选）")
			fmt.Println("   内容：对CIDR网段进行二次扫描的结果")
			fmt.Println("   字段：与step1.csv相同")
			fmt.Println("   可信度：0（空间测绘数据，需要进一步验证）")
			fmt.Println("")
			fmt.Println("⚠️  注意：所有输出文件的可信度均为0，表示数据来源于空间测绘平台，")
			fmt.Println("   建议进行进一步的人工验证和渗透测试确认。")
			fmt.Println("========================")
			fmt.Println("")

			// 重新尝试读取生成的配置文件
			data, err = ioutil.ReadFile(path)
			if err != nil {
				return nil, true, fmt.Errorf("读取生成的配置文件失败: %w", err)
			}
		} else {
			return nil, true, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, true, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 确定目标文件名
	targetFile := cfg.Input.TargetFile
	if targetFile == "" {
		targetFile = "targets.csv" // 兜底文件名
	}

	// 检查目标文件是否存在，如果不存在则生成默认文件
	shouldExit, err := EnsureTargetFileExists(targetFile)
	if err != nil {
		return nil, true, fmt.Errorf("处理目标文件失败: %w", err)
	}
	if shouldExit {
		return nil, true, nil
	}

	// 更新配置中的目标文件名
	cfg.Input.TargetFile = targetFile

	return &cfg, false, nil
}

// generateDefaultConfig 生成默认配置文件
func generateDefaultConfig(path string) error {
	// 构造带注释的默认配置内容
	defaultConfigContent := `# config.yaml

# 各空间测绘平台 API Key 配置（留空表示不使用该平台）
api_keys:
  fofa: ""      # FOFA API Key
  quake: ""     # Quake API Key  
  hunter: ""    # Hunter API Key

# 查询参数设置
query:
  min_ips_per_cidr: 10          # 一个C段最少有几个IP才会被二次扫描；设置为-1时跳过第二轮扫描
  min_urls_per_ip_for_flag: 10  # 同一个IP关联URL超过这个数量，标记为"需要手动扫描"
  interval_seconds: 3          # 每次查询后的间隔时间（秒），防止过于高频扫描导致查询失败

# 输入目标配置
input:
  target_file: "targets.csv"   # 默认读取目标文件路径，可为targets.txt或targets.csv

# 结果输出目录（可选，默认创建在 ./results/yyyyMMdd_xxxxxx）
output:
  base_dir: "./results"
`

	// 写入文件
	if err := ioutil.WriteFile(path, []byte(defaultConfigContent), 0644); err != nil {
		return fmt.Errorf("写入默认配置文件失败: %w", err)
	}

	return nil
}

// EnsureTargetFileExists 确保目标文件存在，如果不存在则生成默认文件
// Returns shouldExit, error
func EnsureTargetFileExists(targetFile string) (bool, error) {
	// 检查文件是否存在
	if _, err := os.Stat(targetFile); err == nil {
		// 文件存在，直接返回
		return false, nil
	} else if !os.IsNotExist(err) {
		// 其他错误
		return true, fmt.Errorf("检查目标文件失败: %w", err)
	}

	// 文件不存在，生成默认文件
	fmt.Printf("目标文件 %s 不存在，正在生成默认文件...\n", targetFile)
	if err := generateDefaultTargetFile(targetFile); err != nil {
		return true, fmt.Errorf("生成默认目标文件失败: %w", err)
	}
	fmt.Printf("默认目标文件已生成: %s\n", targetFile)
	fmt.Println("")
	fmt.Println("=== 目标文件使用说明 ===")
	fmt.Printf("在%s中输入扫描目标，格式如下：\n", targetFile)
	fmt.Println("名称,目标")
	fmt.Println("test,\"test.com\r\n1.1.1.1/24\"")
	fmt.Println("百度,baidu.com")
	fmt.Println("")
	fmt.Println("支持的目标格式：")
	fmt.Println("- 单个IP: 192.168.1.1")
	fmt.Println("- CIDR网段: 192.168.1.0/24")
	fmt.Println("- 域名: example.com")
	fmt.Println("")
	fmt.Println("=== 输出文件说明 ===")
	fmt.Println("程序运行后会在results目录下生成以下文件：")
	fmt.Println("")
	fmt.Println("📁 [时间戳]_step1.csv")
	fmt.Println("   内容：原始扫描结果，包含所有发现的资产信息")
	fmt.Println("   字段：组织代码、域名、主机、协议、URL、IP、端口、状态码、长度、标题、数据来源、可信度")
	fmt.Println("   可信度：0（根据使用者提供的信息做测绘查询，默认可信度最高，可信度标记为0，不做IP密集C段查询直接导出，避免查询过久，可以先用这个文件做工作）")
	fmt.Println("")
	fmt.Println("📁 [时间戳]_step2.csv（可选）")
	fmt.Println("   内容：对CIDR网段进行二次扫描的结果")
	fmt.Println("   字段：与step1.csv相同")
	fmt.Println("   可信度：1、2（除第一轮测绘结果外，针对IP密集C段做二次测绘，其中发现的新增资产，如果IP在第一轮测绘中出现过，则认为是较高可信度资产，可信度标记为1，否则认为可信度较低，标记为2）")
	fmt.Println("")
	fmt.Println("📁 [时间戳]_ip_need_scan.csv")
	fmt.Println("   内容：需要进一步扫描的IP地址汇总")
	fmt.Println("   字段：IP地址、关联URL数量、关联域名、数据来源、生成时间")
	fmt.Println("   可信度：3（上述测绘结果中，对于一个IP地址出现多个url资产，认为这个IP可能存在大量业务，可以考虑进行单独的端口扫描，可信度为3）")
	fmt.Println("")
	fmt.Println("⚠️  注意：所有输出文件的可信度均为0，表示数据来源于空间测绘平台，")
	fmt.Println("   建议进行进一步的人工验证和渗透测试确认。")
	fmt.Println("========================")
	fmt.Println("")

	return true, nil
}

// generateDefaultTargetFile 生成默认目标文件
func generateDefaultTargetFile(targetFile string) error {
	// 生成完全空白的文件，不包含任何内容
	var content string
	if strings.HasSuffix(targetFile, ".csv") {
		content = `,`
	} else {
		// 默认txt格式
		content = `# 扫描目标列表
# 支持格式：单个IP、CIDR网段、域名
# 每行一个目标`
	}

	// 写入文件
	if err := ioutil.WriteFile(targetFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入默认目标文件失败: %w", err)
	}

	return nil
}

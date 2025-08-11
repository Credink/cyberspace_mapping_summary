package model

// TargetEntry 表示 loader.csv 里的一行：单位代号 + 域名/IP
type TargetEntry struct {
	Unit string // 单位代号
	Host string // 域名或 IP
}

// QueryResult 是所有空间测绘平台标准化后的结果结构
type QueryResult struct {
	Unit        string // 所属单位代号
	Domain      string // 域名（不包含IP）
	Host        string // 主机名或IP地址
	Protocol    string // 协议（http/https）
	URL         string // 完整URL
	IP          string // IP地址
	Port        int    // 端口号
	StatusCode  int    // HTTP状态码
	Length      int    // 页面长度
	Title       string // 页面标题
	Source      string // 数据来源平台，例如 quake/fofa/hunter
	Reliability int    // 可信度 0/1/2
}

package loader

import (
	"encoding/csv"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"cyberspace_mapping_summary/internal/model"
)

func ReadTargetsFromCSV(path string) ([]model.TargetEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 尝试检测文件编码并转换为UTF-8
	// 首先尝试UTF-8
	reader := csv.NewReader(file)
	reader.LazyQuotes = true // 允许多行字段
	reader.FieldsPerRecord = -1

	// 读取一小部分内容来检测编码
	file.Seek(0, 0)
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// 检查是否为UTF-8编码
	isUTF8 := true
	for i := 0; i < n; i++ {
		if buf[i] >= 0x80 {
			// 检查UTF-8字节序列
			if i+1 < n && (buf[i]&0xE0) == 0xC0 && (buf[i+1]&0xC0) == 0x80 {
				i++ // 跳过下一个字节
			} else if i+2 < n && (buf[i]&0xF0) == 0xE0 && (buf[i+1]&0xC0) == 0x80 && (buf[i+2]&0xC0) == 0x80 {
				i += 2 // 跳过下两个字节
			} else {
				isUTF8 = false
				break
			}
		}
	}

	// 如果不是UTF-8，尝试GBK编码
	if !isUTF8 {
		file.Seek(0, 0)
		utf8Reader := transform.NewReader(file, simplifiedchinese.GBK.NewDecoder())
		reader = csv.NewReader(utf8Reader)
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1
	} else {
		// 重新定位到文件开头
		file.Seek(0, 0)
		reader = csv.NewReader(file)
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1
	}

	var results []model.TargetEntry

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) < 2 {
			continue // 跳过非法行
		}

		org := strings.TrimSpace(record[0])
		rawHosts := strings.TrimSpace(record[1])

		// 将多行（回车换行）域名拆开
		hosts := strings.Split(rawHosts, "\n")
		for _, h := range hosts {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			results = append(results, model.TargetEntry{
				Unit: org,
				Host: h,
			})
		}
	}

	return results, nil
}

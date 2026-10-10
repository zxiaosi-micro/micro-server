// Package confx 服务配置加载：展开 ${VAR|default} 占位后交 go-zero conf。
//
// 背景：go-zero conf 默认不做环境变量展开（WithEnv 也只走 os.ExpandEnv，
// 不支持默认值语法）；newsvc.sh 模板约定是 ${VAR|default}——环境变量优先、
// 未设置取默认值。本包实现该语义，全部服务统一经此加载配置（S3-06 模板件）。
package confx

import (
	"log"
	"os"
	"regexp"

	"github.com/zeromicro/go-zero/core/conf"
)

// 占位形态：${VAR} / ${VAR|default}（default 可为空串）。
var placeholderRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?:\|([^}]*))?\}`)

// MustLoad 读取文件 → 展开占位 → go-zero conf 解析；失败直接退出（启动期 fail-fast）。
func MustLoad(file string, v any) {
	raw, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("confx: 读配置 %s 失败: %v", file, err)
	}
	if err := LoadBytes(raw, v); err != nil {
		log.Fatalf("confx: 解析配置 %s 失败: %v", file, err)
	}
}

// LoadBytes 展开字节流占位并解析（测试用）。
func LoadBytes(content []byte, v any) error {
	return conf.LoadFromYamlBytes([]byte(Expand(string(content))), v)
}

// Expand 展开 ${VAR|default}：环境变量已设置且非空 → 取 env；否则取默认值（可空）。
func Expand(content string) string {
	return placeholderRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := placeholderRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := os.LookupEnv(sub[1]); ok && v != "" {
			return v
		}
		if len(sub) > 2 {
			return sub[2]
		}
		return ""
	})
}

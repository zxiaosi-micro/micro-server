// confcenter 桥（rules 包访问点）。
package rules

import (
	"micro-server/services/iotingest/internal/confcenter"
	"micro-server/services/iotingest/internal/config"
)

// confcenterCurrent 取 pk 相关规则（当前全局规则集，scope 维度归 ops 判定细化）。
func confcenterCurrent(_ string) []config.RuleConf {
	if rc := confcenter.Current(); rc != nil {
		return rc.Rules
	}
	return nil
}

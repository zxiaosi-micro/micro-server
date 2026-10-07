// Package commoncheck 是 S1-12 的依赖验证样例:
// micro-server 侧引用 micro-common 的最小接线(go.mod require v0.1.0 +
// 根开发用 go.work 本地解析),验证 workspace 编译链路;S2 起各工具
// (envcheck/migrate/seed/keygen 等)按真实需求消费公共库。
package commoncheck

import (
	"fmt"

	"github.com/zxiaosi-micro/micro-common/errcode"
)

// SegmentEcho 返回业务码所属域(公共库 errcode 的最小消费样例)。
// 也用于 S1-12 验收:micro-server 侧 go.mod 引用 + workspace 解析编译通过。
func SegmentEcho(code int64) string {
	return fmt.Sprintf("code=%d domain=%s", code, errcode.DomainOf(code))
}

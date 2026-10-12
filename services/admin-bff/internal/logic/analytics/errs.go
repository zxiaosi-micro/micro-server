// errs.go · admin-bff 错误码段登记（ADR：错误码在各服务 internal/*_err.go 集中登记）。
// analytics 段（150000）在本进程内必须显式注册——analytics 服务的 init 跨进程不生效
// （S11-01 pentest 启动 admin-bff 即 panic 的实测；Register 无返回值，包一层 var 保证
// 先于其他包级 errcode.New 求值——包级 var 依赖序 + 文件名字典序 errs.go < kernel.go）。

package analytics

import "github.com/zxiaosi-micro/micro-common/errcode"

var _ = func() bool {
	errcode.Register(150000, "analytics")
	return true
}()

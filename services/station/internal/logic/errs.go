// station 业务错误码（station 段 70000：micro-common 预注册）。
package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegStation 段 = micro-common 预注册段（S6 场站，勿再 Register）。
const SegStation = errcode.SegStation

var (
	errStationNotFound  = errcode.New(SegStation, 1001, "场站不存在")
	errStationNoUsed    = errcode.New(SegStation, 1002, "场站编号已存在")
	errNameRequired     = errcode.New(SegStation, 1003, "场站名称不能为空")
	errTypeBad          = errcode.New(SegStation, 1004, "场站类型非法（ESS/CHARGING/HESS）")
	errStatusBad        = errcode.New(SegStation, 1005, "非法状态值")
	errBindEmpty        = errcode.New(SegStation, 1006, "绑定清单为空")
	errBindNoDevice     = errcode.New(SegStation, 1007, "绑定项缺 device_id/sn")
	errTopologyBadJson  = errcode.New(SegStation, 1008, "拓扑 nodes/edges 不是合法 JSON")
	errStaffTypeBad     = errcode.New(SegStation, 1009, "人员类型非法（RESIDENT/INSPECTOR/MANAGER）")
	errStaffNotFound    = errcode.New(SegStation, 1010, "场站人员不存在")
	errDeviceNotFound   = errcode.New(SegStation, 1011, "设备不存在或未绑定")
	errDeviceRpcMissing = errcode.New(SegStation, 1013, "device RPC 未配置（SN 归一不可用）")
)

// mustTenant 业务面租户（BFF authz 注入；Saga 消费侧由信封恢复）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}

// isDupKey MySQL 1062 唯一冲突判定。
func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func clampPage(page, size int32) (int, int) {
	p := int(page)
	s := int(size)
	if p < 1 {
		p = 1
	}
	if s < 1 {
		s = 20
	}
	if s > 100 {
		s = 100
	}
	return p, s
}

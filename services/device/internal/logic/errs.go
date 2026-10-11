// device 业务错误码（device 段 80000：资产与 IoT 域，micro-common 预注册）。

package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegDevice 段 = micro-common 预注册段（S6 资产与 IoT，勿再 Register）。
const SegDevice = errcode.SegDevice

var (
	errDeviceNotFound  = errcode.New(SegDevice, 1001, "设备不存在")
	errSnRequired      = errcode.New(SegDevice, 1002, "SN 不能为空")
	errProductRequired = errcode.New(SegDevice, 1004, "product_key 不能为空")
	errImportEmpty     = errcode.New(SegDevice, 1005, "导入清单为空")
	errImportTooMany   = errcode.New(SegDevice, 1006, "单次导入超上限（1000）")
	errEncryptNotReady = errcode.New(SegDevice, 1007, "数据密钥未就绪（MICRO_DATA_KEYS）")
	errStatusBad       = errcode.New(SegDevice, 1008, "非法状态值")
	errTransitionBad   = errcode.New(SegDevice, 1009, "状态迁移不被允许（白名单外）")
	errDeviceActivated = errcode.New(SegDevice, 1010, "设备已激活")
	errDeviceNotOut    = errcode.New(SegDevice, 1011, "设备未出库，不能激活")
	errShadowNotFound  = errcode.New(SegDevice, 1012, "设备影子不存在（未上报过遥测）")
	errDeviceIdOrSn    = errcode.New(SegDevice, 1013, "device_id 与 sn 至少提供一个")
	errCmdTypeBad      = errcode.New(SegDevice, 1014, "指令类型不支持")
	errParamsBadJson   = errcode.New(SegDevice, 1015, "params 不是合法 JSON")
	errFwShaBad        = errcode.New(SegDevice, 1018, "sha256 必须为 64 位 hex")
	errFwSignBad       = errcode.New(SegDevice, 1019, "固件签名校验失败（拒绝入库，FR-IOT-007）")
	errFwSignNotReady  = errcode.New(SegDevice, 1020, "OTA 公钥未配置（拒绝固件入库）")
	errFwNotFound      = errcode.New(SegDevice, 1021, "固件不存在")
	errOtaTaskNotFound = errcode.New(SegDevice, 1022, "OTA 任务不存在")
	errOtaStatusBad    = errcode.New(SegDevice, 1023, "OTA 任务状态不允许该操作")
	errOtaNoTarget     = errcode.New(SegDevice, 1024, "OTA 目标设备为空")
	errOtaRollbackMiss = errcode.New(SegDevice, 1025, "未配置回滚固件，不能回滚")
	errTopologyTypeBad = errcode.New(SegDevice, 1026, "拓扑节点类型非法（CELL/MODULE/PACK/CHARGER）")
	errTopologyTooDeep = errcode.New(SegDevice, 1027, "拓扑层级超过 4 级")
	errProvisionFailed = errcode.New(SegDevice, 1028, "EMQX 凭证写入失败")

	// S10-01 退役评估（梯次利用/回收）。
	errRetireNotFound      = errcode.New(SegDevice, 1029, "退役评估不存在")
	errRetireDeviceBad     = errcode.New(SegDevice, 1030, "设备状态不允许发起退役评估（须 ACTIVATED/RETIRED）")
	errRetirePendingDup    = errcode.New(SegDevice, 1031, "该设备存在未完结的退役评估")
	errRetireResultDup     = errcode.New(SegDevice, 1032, "检测结果已提交，勿重复提交")
	errRetireNotAssessed   = errcode.New(SegDevice, 1033, "尚未提交检测结果，无法评级处置")
	errRetireSohBad        = errcode.New(SegDevice, 1034, "SOH 数值不合法（0~100）")
	errRetireBranchBad     = errcode.New(SegDevice, 1035, "处置分支与评级不符（A→RESALE/B→DERATE/C→RECYCLE）")
	_                      = errcode.New(SegDevice, 1036, "回收单不存在") // 预留（回收单详情接口）
	errRetireSourceBad     = errcode.New(SegDevice, 1037, "检测来源不合法（MANUAL/TELEMETRY/BENCH）")
	errRetireDecimalBad    = errcode.New(SegDevice, 1038, "数值字段不是合法 DECIMAL 字符串")
	errRetireTransitionBad = errcode.New(SegDevice, 1039, "设备状态已变化（CAS 失败），请重试")
	errRecycleDup          = errcode.New(SegDevice, 1040, "该评估已生成回收单")
)

// mustTenant 业务面租户（BFF authz 注入）。
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

// clampPage 分页上限收敛。 page<1→1, size<1→20, size>100→100
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

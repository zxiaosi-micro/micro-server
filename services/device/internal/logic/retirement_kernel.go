// 退役评估/回收 logic（S10-01 梯次利用 + 回收：评级判定 → 三分支处置）。
//
// 评级定稿（v4/03 阶段 10）：
//   A（SOH≥80 且循环<1500 且一致性达标）→ RESALE：RETIRED→IN_STOCK 二次销售
//     （新单走 order→contract CreateFromOrder 天然建新 PENDING 质保，A 级新质保即生效）；
//   B（70≤SOH<80）→ DERATE：降容备电不给销售质保（终态 RETIRED，设备状态不变）；
//   C（SOH<70 或安全不过）→ RECYCLE：→SCRAPPED + 回收单（回收商/残值/凭证/监管溯源码）。
// 检测数据留痕 device_soh_snapshot；检测工单复用 ops 体系（inspection_ticket_no 关联）。

package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// retireGradeWhitelist RESALE 的前置状态（梯次利用白名单对：RETIRED→IN_STOCK）。
const retireResaleFrom = "RETIRED"

// retireScrapFroms RECYCLE 允许的报废前置状态（对齐 transitionAllow["SCRAPPED"]）。
var retireScrapFroms = map[string]bool{"IN_STOCK": true, "OUT": true, "ACTIVATED": true, "RETIRED": true}

// CreateRetirementAssessment 发起退役评估（ACTIVATED/RETIRED 设备；一设备同时仅一笔未完结）。
func (l *CreateRetirementAssessmentLogic) CreateRetirementAssessment(in *pb.CreateRetirementAssessmentReq) (*pb.CreateRetirementAssessmentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.DeviceId <= 0 {
		return nil, errDeviceNotFound
	}
	d, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	if d.Status != "ACTIVATED" && d.Status != "RETIRED" {
		return nil, errRetireDeviceBad
	}
	if pending, perr := l.svcCtx.Models.Retirement.HasPendingByDevice(l.ctx, tid, in.DeviceId); perr != nil {
		return nil, perr
	} else if pending {
		return nil, errRetirePendingDup
	}
	op := opUID(l.ctx)
	round, err := l.svcCtx.Models.Retirement.NextRoundByDevice(l.ctx, tid, in.DeviceId)
	if err != nil {
		return nil, err
	}

	assessmentId := l.svcCtx.Snowflake.MustNextID()
	assessmentNo := fmt.Sprintf("RA%d", assessmentId)
	row := &model.RetirementAssessment{
		AssessmentId:       assessmentId,
		AssessmentNo:       assessmentNo,
		DeviceId:           d.DeviceId,
		Sn:                 d.Sn,
		Round:              round,
		Disposition:        "PENDING",
		InspectionTicketNo: toNullString(in.InspectionTicketNo),
		Remark:             toNullString(in.Remark),
		TenantId:           tid,
		CreatedBy:          toNullInt64(op),
		UpdatedBy:          toNullInt64(op),
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return l.svcCtx.Models.Retirement.InsertTx(ctx, session, row)
	})
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("retirement assessment created no=%s sn=%s round=%d ticket=%s",
		assessmentNo, d.Sn, round, in.InspectionTicketNo)
	return &pb.CreateRetirementAssessmentResp{
		AssessmentId: assessmentId,
		AssessmentNo: assessmentNo,
		Round:        int32(round),
	}, nil
}

// SubmitRetirementResult 检测结果录入（ops 检测工单回填）：SOH 快照留痕 + 评级判定。
func (l *SubmitRetirementResultLogic) SubmitRetirementResult(in *pb.SubmitRetirementResultReq) (*pb.SubmitRetirementResultResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	ra, err := l.svcCtx.Models.Retirement.FindOneScoped(l.ctx, tid, in.AssessmentId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errRetireNotFound
		}
		return nil, err
	}
	if ra.Grade.Valid {
		return nil, errRetireResultDup
	}
	soh, err := parseDecimal(in.Soh)
	if err != nil {
		return nil, err
	}
	if soh < 0 || soh > 100 {
		return nil, errRetireSohBad
	}
	resistance, err := parseDecimal(in.ResistanceMohm)
	if err != nil {
		return nil, err
	}
	source := in.Source
	if source == "" {
		source = "MANUAL"
	}
	if source != "MANUAL" && source != "TELEMETRY" && source != "BENCH" {
		return nil, errRetireSourceBad
	}

	// 安全检测：任一不过即安全失败；未录项视为通过。
	safetyPass := true
	safetyJson := "[]"
	if len(in.SafetyItems) > 0 {
		items := make([]map[string]any, 0, len(in.SafetyItems))
		for _, s := range in.SafetyItems {
			if !s.Pass {
				safetyPass = false
			}
			items = append(items, map[string]any{"item": s.Item, "pass": s.Pass})
		}
		raw, jerr := json.Marshal(items)
		if jerr != nil {
			return nil, jerr
		}
		safetyJson = string(raw)
	}
	grade, reason := retireGrade(soh, int64(in.CycleCount), in.ConsistencyOk, safetyPass)

	measuredAt := time.Now()
	if in.MeasuredAt > 0 {
		measuredAt = time.UnixMilli(in.MeasuredAt)
	}
	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		ra.Soh = soh
		ra.CycleCount = int64(in.CycleCount)
		ra.ResistanceMohm = nullFloat(resistance)
		ra.ConsistencyOk = boolToInt64(in.ConsistencyOk)
		ra.SafetyJson = sql.NullString{String: safetyJson, Valid: true}
		ra.Grade = sql.NullString{String: grade, Valid: true}
		ra.UpdatedBy = toNullInt64(op)
		if err := l.svcCtx.Models.Retirement.UpdateTx(ctx, session, ra); err != nil {
			return err
		}
		return l.svcCtx.Models.Soh.InsertTx(ctx, session, &model.DeviceSohSnapshot{
			SnapshotId:     l.svcCtx.Snowflake.MustNextID(),
			AssessmentId:   ra.AssessmentId,
			DeviceId:       ra.DeviceId,
			Sn:             ra.Sn,
			Soh:            soh,
			CycleCount:     int64(in.CycleCount),
			ResistanceMohm: nullFloat(resistance),
			MeasuredAt:     measuredAt,
			Source:         source,
			TenantId:       tid,
		})
	})
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("retirement result submitted no=%s sn=%s soh=%.2f grade=%s", ra.AssessmentNo, ra.Sn, soh, grade)
	return &pb.SubmitRetirementResultResp{Grade: grade, Reason: reason}, nil
}

// DecideRetirement 评级处置三分支（B 可显式降级 RECYCLE，其余按 grade 强校验）。
func (l *DecideRetirementLogic) DecideRetirement(in *pb.DecideRetirementReq) (*pb.DecideRetirementResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	ra, err := l.svcCtx.Models.Retirement.FindOneScoped(l.ctx, tid, in.AssessmentId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errRetireNotFound
		}
		return nil, err
	}
	if !ra.Grade.Valid {
		return nil, errRetireNotAssessed
	}
	grade := ra.Grade.String
	target := in.Target
	if target == "" {
		switch grade {
		case "A":
			target = "RESALE"
		case "B":
			target = "DERATE"
		default:
			target = "RECYCLE"
		}
	}
	// 分支与评级强校验（B 可降级回收：降容备电也可直接报废，业务允许）。
	switch {
	case target == "RESALE" && grade != "A":
		return nil, errRetireBranchBad
	case target == "DERATE" && grade == "C":
		return nil, errRetireBranchBad
	case target != "RESALE" && target != "DERATE" && target != "RECYCLE":
		return nil, errRetireBranchBad
	}

	op := opUID(l.ctx)
	resp := &pb.DecideRetirementResp{Grade: grade, Disposition: target}
	if target == "RECYCLE" {
		resp.Disposition = "RECYCLED" // 与落库处置值一致（RESALE/DERATE 天然一致）
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		switch target {
		case "RESALE":
			n, cerr := l.svcCtx.Models.Device.CasStatusTx(ctx, session, tid, ra.DeviceId, retireResaleFrom, "IN_STOCK")
			if cerr != nil {
				return cerr
			}
			if n == 0 {
				return errRetireTransitionBad
			}
			if cerr := lifecycleAppendTx(ctx, session, l.svcCtx, ra.DeviceId, ra.Sn, "RETIRED", "IN_STOCK",
				"device.resale", fmt.Sprintf("resale:%s", ra.AssessmentNo), "梯次利用 A 级二次销售", tid, op); cerr != nil {
				return cerr
			}
			ra.Disposition = "RESALE"
		case "DERATE":
			// 降容备电：设备终态保持 RETIRED，仅落处置结论。
			ra.Disposition = "DERATE"
		default: // RECYCLE
			d, derr := l.svcCtx.Models.Device.FindOneScoped(ctx, tid, ra.DeviceId)
			if derr != nil {
				return derr
			}
			if !retireScrapFroms[d.Status] {
				return errRetireTransitionBad
			}
			residual, rerr := parseDecimal(in.ResidualValue)
			if rerr != nil {
				return rerr
			}
			recycleId := l.svcCtx.Snowflake.MustNextID()
			recycleNo := fmt.Sprintf("RC%d", recycleId)
			recycler, rerr := parseID(in.RecyclerPartyId)
			if rerr != nil {
				return rerr
			}
			traceCode := fmt.Sprintf("TC%016X", recycleId)
			if cerr := l.svcCtx.Models.Recycle.InsertTx(ctx, session, &model.RecycleOrder{
				RecycleId:       recycleId,
				RecycleNo:       recycleNo,
				AssessmentId:    ra.AssessmentId,
				DeviceId:        ra.DeviceId,
				Sn:              ra.Sn,
				RecyclerPartyId: toNullInt64(recycler),
				ResidualValue:   residual,
				TraceCode:       traceCode,
				Status:          "CREATED",
				Remark:          toNullString(in.Remark),
				TenantId:        tid,
				CreatedBy:       toNullInt64(op),
				UpdatedBy:       toNullInt64(op),
			}); cerr != nil {
				return cerr
			}
			n, cerr := l.svcCtx.Models.Device.CasStatusTx(ctx, session, tid, ra.DeviceId, d.Status, "SCRAPPED")
			if cerr != nil {
				return cerr
			}
			if n == 0 {
				return errRetireTransitionBad
			}
			if cerr := lifecycleAppendTx(ctx, session, l.svcCtx, ra.DeviceId, ra.Sn, d.Status, "SCRAPPED",
				"device.recycled", fmt.Sprintf("recycle:%s", recycleNo), "C 级报废走回收单", tid, op); cerr != nil {
				return cerr
			}
			ra.Disposition = "RECYCLED"
			resp.RecycleNo = recycleNo
			resp.TraceCode = traceCode
		}
		ra.UpdatedBy = toNullInt64(op)
		return l.svcCtx.Models.Retirement.UpdateTx(ctx, session, ra)
	})
	if err != nil {
		return nil, err
	}
	d, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, ra.DeviceId)
	if err == nil {
		resp.DeviceStatus = d.Status
	}
	logx.WithContext(l.ctx).Infof("retirement decided no=%s grade=%s target=%s recycle_no=%s",
		ra.AssessmentNo, grade, target, resp.RecycleNo)
	return resp, nil
}

// ListRetirementAssessment 评估列表。
func (l *ListRetirementAssessmentLogic) ListRetirementAssessment(in *pb.ListRetirementAssessmentReq) (*pb.ListRetirementAssessmentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Grade != "" && in.Grade != "A" && in.Grade != "B" && in.Grade != "C" {
		return nil, errRetireBranchBad
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Retirement.ListPage(l.ctx, tid, in.Keyword, in.Grade, in.Disposition, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListRetirementAssessmentResp{Total: total, List: []*pb.RetirementAssessmentView{}}
	for _, ra := range list {
		resp.List = append(resp.List, assessmentView(ra))
	}
	return resp, nil
}

// CreateRecycleOrder C 级评估补建回收单（DecideRetirement RECYCLE 漏建补录；一评估一单）。
func (l *CreateRecycleOrderLogic) CreateRecycleOrder(in *pb.CreateRecycleOrderReq) (*pb.CreateRecycleOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	ra, err := l.svcCtx.Models.Retirement.FindOneScoped(l.ctx, tid, in.AssessmentId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errRetireNotFound
		}
		return nil, err
	}
	if !ra.Grade.Valid || ra.Grade.String != "C" {
		return nil, errRetireBranchBad
	}
	if ra.Disposition == "RECYCLED" {
		return nil, errRecycleDup
	}
	residual, err := parseDecimal(in.ResidualValue)
	if err != nil {
		return nil, err
	}
	recycler, err := parseID(in.RecyclerPartyId)
	if err != nil {
		return nil, err
	}
	evidence, err := parseID(in.EvidenceFileId)
	if err != nil {
		return nil, err
	}
	op := opUID(l.ctx)
	recycleId := l.svcCtx.Snowflake.MustNextID()
	recycleNo := fmt.Sprintf("RC%d", recycleId)
	traceCode := fmt.Sprintf("TC%016X", recycleId)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := l.svcCtx.Models.Recycle.InsertTx(ctx, session, &model.RecycleOrder{
			RecycleId:       recycleId,
			RecycleNo:       recycleNo,
			AssessmentId:    ra.AssessmentId,
			DeviceId:        ra.DeviceId,
			Sn:              ra.Sn,
			RecyclerPartyId: toNullInt64(recycler),
			ResidualValue:   residual,
			TraceCode:       traceCode,
			EvidenceFileId:  toNullInt64(evidence),
			Status:          "CREATED",
			Remark:          toNullString(in.Remark),
			TenantId:        tid,
			CreatedBy:       toNullInt64(op),
			UpdatedBy:       toNullInt64(op),
		}); err != nil {
			return err
		}
		ra.Disposition = "RECYCLED"
		ra.UpdatedBy = toNullInt64(op)
		return l.svcCtx.Models.Retirement.UpdateTx(ctx, session, ra)
	})
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("recycle order created no=%s sn=%s trace=%s", recycleNo, ra.Sn, traceCode)
	return &pb.CreateRecycleOrderResp{RecycleId: recycleId, RecycleNo: recycleNo, TraceCode: traceCode}, nil
}

// ListRecycleOrder 回收单列表。
func (l *ListRecycleOrderLogic) ListRecycleOrder(in *pb.ListRecycleOrderReq) (*pb.ListRecycleOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Recycle.ListPage(l.ctx, tid, in.Keyword, in.Status, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListRecycleOrderResp{Total: total, List: []*pb.RecycleOrderView{}}
	for _, ro := range list {
		resp.List = append(resp.List, recycleView(ro))
	}
	return resp, nil
}

// ---------- 内部助手 ----------

// retireGrade 评级判定（定稿口径：A=SOH≥80 且循环<1500 且一致性达标；B=SOH∈[70,80)
// 或 SOH≥80 但循环超限/一致性不达标（容量尚可寿命受限，降容备电）；C=SOH<70 或安全不过）。
func retireGrade(soh float64, cycles int64, consistencyOk, safetyPass bool) (grade, reason string) {
	switch {
	case !safetyPass:
		return "C", "安全检测未通过"
	case soh < 70:
		return "C", "SOH<70"
	case soh >= 80 && cycles < 1500 && consistencyOk:
		return "A", "SOH≥80 且循环<1500 且一致性达标"
	case soh < 80:
		return "B", "SOH∈[70,80)：降容备电"
	case cycles >= 1500:
		return "B", "循环数≥1500：降容备电"
	default:
		return "B", "一致性不达标：降容备电"
	}
}

// parseDecimal DECIMAL 字符串解析（空串=0）。
func parseDecimal(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errRetireDecimalBad
	}
	return v, nil
}

// parseID E8 string ID 解析（空串=0）。
func parseID(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errRetireDecimalBad
	}
	return v, nil
}

func nullFloat(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: true}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// assessmentView 行 → proto 视图（DECIMAL string 防精度丢失）。
func assessmentView(ra *model.RetirementAssessment) *pb.RetirementAssessmentView {
	v := &pb.RetirementAssessmentView{
		AssessmentId:  ra.AssessmentId,
		AssessmentNo:  ra.AssessmentNo,
		DeviceId:      ra.DeviceId,
		Sn:            ra.Sn,
		Round:         int32(ra.Round),
		Soh:           decimalStr(ra.Soh),
		CycleCount:    int32(ra.CycleCount),
		ConsistencyOk: ra.ConsistencyOk == 1,
		Grade:         ra.Grade.String,
		Disposition:   ra.Disposition,
		Remark:        ra.Remark.String,
		CreatedAt:     ra.CreatedAt.UnixMilli(),
	}
	if ra.ResistanceMohm.Valid {
		v.ResistanceMohm = decimalStr(ra.ResistanceMohm.Float64)
	}
	if ra.SafetyJson.Valid {
		v.SafetyJson = ra.SafetyJson.String
	}
	if ra.InspectionTicketNo.Valid {
		v.InspectionTicketNo = ra.InspectionTicketNo.String
	}
	return v
}

func recycleView(ro *model.RecycleOrder) *pb.RecycleOrderView {
	v := &pb.RecycleOrderView{
		RecycleId:     ro.RecycleId,
		RecycleNo:     ro.RecycleNo,
		AssessmentId:  ro.AssessmentId,
		DeviceId:      ro.DeviceId,
		Sn:            ro.Sn,
		ResidualValue: decimalStr(ro.ResidualValue),
		TraceCode:     ro.TraceCode,
		Status:        ro.Status,
		Remark:        ro.Remark.String,
		CreatedAt:     ro.CreatedAt.UnixMilli(),
	}
	if ro.RecyclerPartyId.Valid {
		v.RecyclerPartyId = strconv.FormatInt(ro.RecyclerPartyId.Int64, 10)
	}
	if ro.EvidenceFileId.Valid {
		v.EvidenceFileId = strconv.FormatInt(ro.EvidenceFileId.Int64, 10)
	}
	return v
}

// decimalStr DECIMAL 值 → 字符串（整数去尾零）。
func decimalStr(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

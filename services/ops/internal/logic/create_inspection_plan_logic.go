package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
	stationpb "micro-server/services/station/pb"
	"time"
)

type CreateInspectionPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateInspectionPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInspectionPlanLogic {
	return &CreateInspectionPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 巡检（FR-OPS-007）----
func (l *CreateInspectionPlanLogic) CreateInspectionPlan(in *pb.CreateInspectionPlanReq) (*pb.CreateInspectionPlanResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	next, err := nextExecAt(time.Now(), in.CronExpr)
	if err != nil {
		return nil, err
	}
	stationName := ""
	if l.svcCtx.Station != nil {
		if st, err := l.svcCtx.Station.GetStation(l.ctx, &stationpb.GetStationReq{StationId: in.StationId}); err == nil && st.Station != nil {
			stationName = st.Station.Name
		}
	}
	row := &model.InspectionPlan{
		PlanId:      l.svcCtx.Snowflake.MustNextID(),
		StationId:   in.StationId,
		StationName: stationName,
		Name:        in.Name,
		TemplateId:  in.TemplateId,
		CronExpr:    in.CronExpr,
		Enabled:     1,
		NextExecAt:  next,
		TenantId:    tid,
		CreatedBy:   nullInt64(ctxkit.UID(l.ctx)),
	}
	if err := l.svcCtx.Models.InsPlan.InsertTx(l.ctx, l.svcCtx.Conn, row); err != nil {
		if isDupKey(err) {
			return nil, errInspectNotFound // uk_station_name 冲突：同名计划已存在
		}
		return nil, err
	}
	return &pb.CreateInspectionPlanResp{PlanId: row.PlanId}, nil
}

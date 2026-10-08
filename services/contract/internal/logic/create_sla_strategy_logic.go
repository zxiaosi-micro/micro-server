package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateSlaStrategyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSlaStrategyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSlaStrategyLogic {
	return &CreateSlaStrategyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateSlaStrategyLogic) CreateSlaStrategy(in *pb.CreateSlaStrategyReq) (*pb.CreateSlaStrategyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Code == "" || in.Name == "" || in.Level == "" {
		return nil, errStrategyCodeUsed.WithMsg("code/name/level 必填")
	}
	if in.ResponseMinutes <= 0 || in.ResolveMinutes <= 0 {
		return nil, errStrategyCodeUsed.WithMsg("响应/解决时限必须大于 0")
	}
	strategyId := l.svcCtx.Snowflake.MustNextID()
	s := &model.SlaStrategy{
		StrategyId: strategyId, Code: in.Code, Name: in.Name, Level: in.Level,
		ResponseMinutes: int64(in.ResponseMinutes), ResolveMinutes: int64(in.ResolveMinutes),
		TenantId: tid, CreatedBy: sqlInt64(ctxkit.UID(l.ctx)), UpdatedBy: sqlInt64(ctxkit.UID(l.ctx)),
	}
	if err := l.svcCtx.Models.Sla.InsertTx(l.ctx, s); err != nil {
		if isDupKey(err) {
			return nil, errStrategyCodeUsed
		}
		return nil, err
	}
	return &pb.CreateSlaStrategyResp{StrategyId: strategyId}, nil
}

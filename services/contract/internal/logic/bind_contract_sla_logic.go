package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type BindContractSlaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindContractSlaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindContractSlaLogic {
	return &BindContractSlaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BindContractSlaLogic) BindContractSla(in *pb.BindContractSlaReq) (*pb.BindContractSlaResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	c, err := l.svcCtx.Models.Contract.FindOneByNo(l.ctx, tid, in.ContractNo)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errContractNotFound
		}
		return nil, err
	}
	s, err := l.svcCtx.Models.Sla.FindOneScoped(l.ctx, tid, in.StrategyId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStrategyNotFound
		}
		return nil, err
	}
	// 绑定即快照冻结（ops 计时按快照，FR-CTR-007）
	snapshot := mustJSON(map[string]any{
		"strategy_id": s.StrategyId, "code": s.Code, "level": s.Level,
		"response_minutes": s.ResponseMinutes, "resolve_minutes": s.ResolveMinutes,
	})
	bind := &model.ContractSla{
		BindId: l.svcCtx.Snowflake.MustNextID(), ContractId: c.ContractId,
		StrategyId: s.StrategyId, Snapshot: snapshot,
		TenantId: tid, CreatedBy: sqlInt64(ctxkit.UID(l.ctx)), UpdatedBy: sqlInt64(ctxkit.UID(l.ctx)),
	}
	if err := l.svcCtx.Models.ContractSla.InsertTx(l.ctx, l.svcCtx.Conn, bind); err != nil {
		if isDupKey(err) {
			return nil, errStrategyCodeUsed.WithMsg("该合同已绑定 SLA 策略")
		}
		return nil, err
	}
	return &pb.BindContractSlaResp{}, nil
}

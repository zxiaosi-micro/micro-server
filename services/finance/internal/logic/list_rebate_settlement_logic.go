package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRebateSettlementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRebateSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRebateSettlementLogic {
	return &ListRebateSettlementLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRebateSettlementLogic) ListRebateSettlement(in *pb.ListRebateSettlementReq) (*pb.ListRebateSettlementResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Period != "" {
		if _, _, err := parseRebatePeriod(in.Period); err != nil {
			return nil, err
		}
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Rebate.ListPage(l.ctx, tid, in.Period, in.Status, in.PartyId, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListRebateSettlementResp{Total: total, List: []*pb.RebateSettlementItem{}}
	for _, r := range list {
		resp.List = append(resp.List, rebateItem(r))
	}
	return resp, nil
}

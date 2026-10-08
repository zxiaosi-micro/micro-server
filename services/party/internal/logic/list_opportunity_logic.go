package logic

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListOpportunityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOpportunityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOpportunityLogic {
	return &ListOpportunityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListOpportunityLogic) ListOpportunity(in *pb.ListOpportunityReq) (*pb.ListOpportunityResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Opportunity.FindPage(l.ctx, tid, in.PartyId, in.Stage, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListOpportunityResp{Total: total}
	for _, o := range list {
		resp.List = append(resp.List, opportunityItem(o))
	}
	return resp, nil
}

func opportunityItem(o *model.Opportunity) *pb.OpportunityItem {
	amount := ""
	if o.Amount.Valid {
		amount = fmt.Sprintf("%.2f", math.Round(o.Amount.Float64*100)/100)
	}
	return &pb.OpportunityItem{
		OpportunityId:     o.OpportunityId,
		PartyId:           o.PartyId,
		Title:             o.Title,
		Stage:             o.Stage,
		Amount:            amount,
		ExpectedCloseDate: nullTimeMilli(o.ExpectedCloseDate),
		Remark:            o.Remark.String,
		CreatedAt:         o.CreatedAt.UnixMilli(),
		UpdatedAt:         o.UpdatedAt.UnixMilli(),
	}
}

var _ = sql.NullString{}

package logic

import (
	"context"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetPartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPartyLogic {
	return &GetPartyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetPartyLogic) GetParty(in *pb.GetPartyReq) (*pb.GetPartyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	p, err := l.svcCtx.Models.Party.FindOne(l.ctx, tid, in.PartyId)
	if err != nil {
		return nil, partyErr(err)
	}
	return &pb.GetPartyResp{Party: partyItem(p)}, nil
}

// partyItem 模型 → proto（Null 类型展平；时间为 UnixMilli，E8）。
func partyItem(p *model.Party) *pb.PartyItem {
	return &pb.PartyItem{
		PartyId:    p.PartyId,
		Name:       p.Name,
		Type:       p.Type,
		Status:     int32(p.Status),
		CreditCode: p.CreditCode.String,
		Region:     p.Region.String,
		Address:    p.Address.String,
		Remark:     p.Remark.String,
		CreatedAt:  p.CreatedAt.UnixMilli(),
		UpdatedAt:  p.UpdatedAt.UnixMilli(),
	}
}

// partyErr 统一把 model.ErrNotFound 映射为业务码。
func partyErr(err error) error {
	if err == sqlx.ErrNotFound {
		return errPartyNotFound
	}
	return errcode.Internal.WithCause(err)
}

package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type BindPartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindPartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPartyLogic {
	return &BindPartyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindParty 统一账号-参与方绑定（S4 party 服务消费；账号←→参与方 1:N 场景由参与方侧聚合）。
func (l *BindPartyLogic) BindParty(in *pb.BindPartyReq) (*pb.BindPartyResp, error) {
	if in.Uid <= 0 || in.PartyId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid/party_id 必填")
	}
	tid, err := l.svcCtx.Models.User.FindTenantByUID(l.ctx, in.Uid)
	if err != nil {
		return nil, errUserNotFound
	}
	if err := l.svcCtx.Models.User.UpdateParty(l.ctx, tid, in.Uid, in.PartyId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.BindPartyResp{}, nil
}

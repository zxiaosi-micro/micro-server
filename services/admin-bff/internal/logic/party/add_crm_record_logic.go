// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddCrmRecordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 追加跟进(perm: party:crm:create)
func NewAddCrmRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddCrmRecordLogic {
	return &AddCrmRecordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddCrmRecordLogic) AddCrmRecord(req *types.CrmRecordCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.AddCrmRecord(l.ctx, &partyPb.AddCrmRecordReq{
		PartyId: parseID(req.Id), Content: req.Content, NextFollowAt: req.NextFollowAt,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

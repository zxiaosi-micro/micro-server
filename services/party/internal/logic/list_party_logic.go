package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListPartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPartyLogic {
	return &ListPartyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListPartyLogic) ListParty(in *pb.ListPartyReq) (*pb.ListPartyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Party.FindPage(l.ctx, tid, in.Keyword, in.Type, int64(in.Status), page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListPartyResp{Total: total}
	for _, p := range list {
		resp.List = append(resp.List, partyItem(p))
	}
	return resp, nil
}

// clampPage 分页上限收敛（size 上限 100，与前端 usePaged 同源约束）。
func clampPage(page, size int64) (int64, int64) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

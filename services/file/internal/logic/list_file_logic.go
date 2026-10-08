package logic

import (
	"context"

	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFileLogic {
	return &ListFileLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListFileLogic) ListFile(in *pb.ListFileReq) (*pb.ListFileResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.FileMeta.FindPage(l.ctx, tid, in.BizType, in.BizId, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListFileResp{Total: total}
	for _, m := range list {
		resp.List = append(resp.List, metaItem(m))
	}
	return resp, nil
}

// clampPage 分页上限收敛（size 上限 100）。
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

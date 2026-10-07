package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListOrgsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOrgsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrgsLogic {
	return &ListOrgsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListOrgs 组织全量平铺（前端组树；组织规模有限不分页）。
func (l *ListOrgsLogic) ListOrgs(in *pb.ListOrgsReq) (*pb.ListOrgsResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Org.FindAllByTenant(l.ctx, tid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.OrgItem, 0, len(list))
	for _, o := range list {
		out = append(out, &pb.OrgItem{
			OrgId:    o.OrgId,
			ParentId: o.ParentId,
			Name:     o.Name,
			Sort:     int32(o.Sort),
		})
	}
	return &pb.ListOrgsResp{List: out}, nil
}

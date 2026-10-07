package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRoles 角色列表（分页上限 100）。
func (l *ListRolesLogic) ListRoles(in *pb.ListRolesReq) (*pb.ListRolesResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := normalizePage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Role.FindPage(l.ctx, tid, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.RoleItem, 0, len(list))
	for _, r := range list {
		out = append(out, &pb.RoleItem{
			RoleId:    r.RoleId,
			Code:      r.Code,
			Name:      r.Name,
			DataScope: r.DataScope.String,
			Remark:    r.Remark.String,
			CreatedAt: r.CreatedAt.UnixMilli(),
		})
	}
	return &pb.ListRolesResp{List: out, Total: total}, nil
}

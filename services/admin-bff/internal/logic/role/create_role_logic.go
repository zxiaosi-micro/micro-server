package role

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRoleLogic {
	return &CreateRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateRoleLogic) CreateRole(req *types.RoleCreateReq) (*types.SimpleResp, error) {
	resp, err := l.svcCtx.Identity.CreateRole(l.ctx, &pb.CreateRoleReq{
		Code: req.Code, Name: req.Name, DataScope: req.DataScope,
		Remark: req.Remark, MenuIds: parseIDs(req.MenuIDs),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(resp.RoleId, 10)}, nil
}

func parseIDs(list []string) []int64 {
	out := make([]int64, 0, len(list))
	for _, s := range list {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

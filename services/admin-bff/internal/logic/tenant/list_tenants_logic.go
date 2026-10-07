package tenant

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTenantsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTenantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTenantsLogic {
	return &ListTenantsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListTenants 租户列表（平台面）。
func (l *ListTenantsLogic) ListTenants(req *types.TenantListReq) (*types.TenantListResp, error) {
	resp, err := l.svcCtx.Identity.ListTenants(l.ctx, &pb.ListTenantsReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	list := make([]types.TenantItem, 0, len(resp.List))
	for _, t := range resp.List {
		list = append(list, types.TenantItem{
			TenantID:   strconv.FormatInt(t.TenantId, 10),
			TenantCode: t.TenantCode,
			Name:       t.Name,
			Plan:       t.Plan,
			Quota:      t.Quota,
			Status:     int(t.Status),
		})
	}
	return &types.TenantListResp{List: list, Total: resp.Total}, nil
}

package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListTenantsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTenantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTenantsLogic {
	return &ListTenantsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListTenants 租户列表（平台面，Exempt 登记豁免；分页上限 100）。
func (l *ListTenantsLogic) ListTenants(in *pb.ListTenantsReq) (*pb.ListTenantsResp, error) {
	page, size := normalizePage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Tenant.FindPage(l.ctx, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.TenantItem, 0, len(list))
	for _, t := range list {
		out = append(out, &pb.TenantItem{
			TenantId:   t.TenantId,
			TenantCode: t.TenantCode,
			Name:       t.Name,
			Plan:       t.Plan,
			Quota:      t.Quota.String,
			Status:     int32(t.Status),
		})
	}
	return &pb.ListTenantsResp{List: out, Total: total}, nil
}

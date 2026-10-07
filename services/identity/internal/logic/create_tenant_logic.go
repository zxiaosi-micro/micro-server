package logic

import (
	"context"
	"database/sql"
	"time"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateTenantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTenantLogic {
	return &CreateTenantLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateTenant 建租户（tenant_code UK 冲突 → 1101012；quota 空 = "{}"）。
func (l *CreateTenantLogic) CreateTenant(in *pb.CreateTenantReq) (*pb.CreateTenantResp, error) {
	if in.TenantCode == "" || in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_code/name 必填")
	}
	quota := in.Quota
	if quota == "" {
		quota = "{}"
	}
	plan := in.Plan
	if plan == "" {
		plan = "STANDARD"
	}
	tid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	_, err := l.svcCtx.Models.Tenant.Insert(l.ctx, &model.Tenant{
		TenantId:   tid,
		TenantCode: in.TenantCode,
		Name:       in.Name,
		Plan:       plan,
		Quota:      sql.NullString{String: quota, Valid: true},
		Status:     1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errTenantCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	l.Infof("建租户 tid=%d code=%s", tid, in.TenantCode)
	return &pb.CreateTenantResp{TenantId: tid}, nil
}

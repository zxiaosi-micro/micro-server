package logic

import (
	"context"
	"time"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateOrgLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrgLogic {
	return &CreateOrgLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrg 建组织（parent_id=0 为根；父节点必须存在且同租户）。
func (l *CreateOrgLogic) CreateOrg(in *pb.CreateOrgReq) (*pb.CreateOrgResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("name 必填")
	}
	if in.ParentId > 0 {
		if _, err := l.svcCtx.Models.Org.FindOne(l.ctx, tid, in.ParentId); err != nil {
			return nil, errOrgNotFound
		}
	}
	oid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Org.Insert(l.ctx, &model.Org{
		OrgId:     oid,
		ParentId:  in.ParentId,
		Name:      in.Name,
		Sort:      int64(in.Sort),
		TenantId:  tid,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateOrgResp{OrgId: oid}, nil
}

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

type CreateRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRoleLogic {
	return &CreateRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateRole 建角色：(tenant_id, code) UK 冲突 → 1101008；菜单权限树随建随绑。
func (l *CreateRoleLogic) CreateRole(in *pb.CreateRoleReq) (*pb.CreateRoleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Code == "" || in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("code/name 必填")
	}
	dataScope := in.DataScope
	if dataScope == "" {
		dataScope = "{\"type\":\"SELF\"}"
	}
	rid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Role.Insert(l.ctx, &model.Role{
		RoleId:    rid,
		Code:      in.Code,
		Name:      in.Name,
		DataScope: toNullString(dataScope),
		Remark:    toNullString(in.Remark),
		TenantId:  tid,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errRoleCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	if len(in.MenuIds) > 0 {
		if err := l.svcCtx.Models.RoleMenu.Replace(l.ctx, rid, in.MenuIds, tid, op); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
	}
	l.Infof("建角色 rid=%d code=%s by=%d", rid, in.Code, op)
	return &pb.CreateRoleResp{RoleId: rid}, nil
}

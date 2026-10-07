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

type CreateMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuLogic {
	return &CreateMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateMenu 建菜单（perm_code (tenant_id,perm_code) UK 冲突 → 1101009）。
func (l *CreateMenuLogic) CreateMenu(in *pb.CreateMenuReq) (*pb.CreateMenuResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" || in.Type < 1 || in.Type > 4 {
		return nil, errcode.ErrBadRequest.WithMsg("name 必填；type ∈ 1~4")
	}
	if in.ParentId > 0 {
		if _, err := l.svcCtx.Models.Menu.FindOne(l.ctx, tid, in.ParentId); err != nil {
			return nil, errMenuNotFound
		}
	}
	mid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Menu.Insert(l.ctx, &model.Menu{
		MenuId:    mid,
		ParentId:  in.ParentId,
		Name:      in.Name,
		Type:      int64(in.Type),
		PermCode:  toNullString(in.PermCode),
		Path:      toNullString(in.Path),
		Icon:      toNullString(in.Icon),
		Sort:      int64(in.Sort),
		Status:    1,
		TenantId:  tid,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errPermCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateMenuResp{MenuId: mid}, nil
}

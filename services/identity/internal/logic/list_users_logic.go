package logic

import (
	"context"
	"encoding/json"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListUsers 用户列表（列=姓名/手机号脱敏/部门/端标签/状态；分页上限 100）。
func (l *ListUsersLogic) ListUsers(in *pb.ListUsersReq) (*pb.ListUsersResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := normalizePage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.User.FindPage(l.ctx, tid, in.Keyword, int64(in.Status), page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.UserItem, 0, len(list))
	for _, u := range list {
		item := &pb.UserItem{
			Uid:       u.UserId,
			OrgId:     u.OrgId.Int64,
			PartyId:   u.PartyId.Int64,
			Nickname:  u.Nickname,
			Status:    int32(u.Status),
			CreatedAt: u.CreatedAt.UnixMilli(),
		}
		if u.LockedUntil.Valid {
			item.LockedUntil = u.LockedUntil.Time.UnixMilli()
		}
		var types []string
		_ = json.Unmarshal([]byte(u.Types), &types)
		item.Types = types
		if u.Mobile.Valid && u.Mobile.String != "" {
			if plain, derr := l.svcCtx.Encryptor.Decrypt(u.Mobile.String); derr == nil {
				item.MobileMasked = crypto.MaskPhone(string(plain))
			}
		}
		if u.Email.Valid && u.Email.String != "" {
			if plain, derr := l.svcCtx.Encryptor.Decrypt(u.Email.String); derr == nil {
				item.EmailMasked = crypto.MaskEmail(string(plain))
			}
		}
		out = append(out, item)
	}
	return &pb.ListUsersResp{List: out, Total: total}, nil
}

// normalizePage 分页参数归一（size 上限 100，与前端 usePaged 同源约束）。
func normalizePage(page, size int64) (int64, int64) {
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

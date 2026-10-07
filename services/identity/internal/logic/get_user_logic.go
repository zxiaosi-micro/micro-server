package logic

import (
	"context"
	"encoding/json"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUser 用户详情（/auth/me 与管理页编辑回显）。
// 手机号/邮箱解密后脱敏展示；role_codes 回显绑定角色。
func (l *GetUserLogic) GetUser(in *pb.GetUserReq) (*pb.GetUserResp, error) {
	uid := in.Uid
	if uid == 0 {
		uid = opUID(l.ctx)
	}
	if uid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	u, err := l.svcCtx.Models.User.FindOne(l.ctx, tid, uid)
	if err != nil {
		return nil, errUserNotFound
	}
	out := &pb.UserItem{
		Uid:      u.UserId,
		OrgId:    u.OrgId.Int64,
		PartyId:  u.PartyId.Int64,
		Nickname: u.Nickname,
		Status:   int32(u.Status),
	}
	if u.LockedUntil.Valid {
		out.LockedUntil = u.LockedUntil.Time.UnixMilli()
	}
	var types []string
	_ = json.Unmarshal([]byte(u.Types), &types)
	out.Types = types
	if u.Mobile.Valid && u.Mobile.String != "" {
		if plain, derr := l.svcCtx.Encryptor.Decrypt(u.Mobile.String); derr == nil {
			out.MobileMasked = crypto.MaskPhone(string(plain))
		}
	}
	if u.Email.Valid && u.Email.String != "" {
		if plain, derr := l.svcCtx.Encryptor.Decrypt(u.Email.String); derr == nil {
			out.EmailMasked = crypto.MaskEmail(string(plain))
		}
	}
	roleIds, err := l.svcCtx.Models.UserRole.FindRoleIdsByUser(l.ctx, tid, uid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out.RoleCodes = l.roleCodes(tid, roleIds)
	out.CreatedAt = u.CreatedAt.UnixMilli()
	return &pb.GetUserResp{User: out}, nil
}

// roleCodes 角色 ID → 角色码（脏绑定跳过）。
func (l *GetUserLogic) roleCodes(tid int64, roleIds []int64) []string {
	codes := make([]string, 0, len(roleIds))
	for _, rid := range roleIds {
		role, err := l.svcCtx.Models.Role.FindOne(l.ctx, tid, rid)
		if err != nil {
			if err == model.ErrNotFound {
				continue
			}
			continue
		}
		codes = append(codes, role.Code)
	}
	return codes
}

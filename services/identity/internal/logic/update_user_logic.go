package logic

import (
	"context"
	"database/sql"
	"encoding/json"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserLogic {
	return &UpdateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateUser 编辑用户：mobile/email 变更走加密+hash 重写（UK 冲突显式报 11001/11002）；
// role_ids 非空时全删全插并刷新 auth_cache。
func (l *UpdateUserLogic) UpdateUser(in *pb.UpdateUserReq) (*pb.UpdateUserResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Uid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	cur, err := l.svcCtx.Models.User.FindOne(l.ctx, tid, in.Uid)
	if err != nil {
		return nil, errUserNotFound
	}

	// mobile/email 变更（空串=不变更）：加密 + hash，先查 UK 防冲突（1062 兜底在后）
	var mobile, mobileHash, email, emailHash sql.NullString
	if in.Mobile != "" {
		if cur.MobileHash.Valid && cur.MobileHash.String == l.hashIndex(in.Mobile) {
			mobile, mobileHash = cur.Mobile, cur.MobileHash // 未变
		} else {
			c, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Mobile))
			if err != nil {
				return nil, errcode.Internal.WithCause(err)
			}
			mobile = toNullString(c)
			mobileHash = toNullString(l.hashIndex(in.Mobile))
			if other, err := l.svcCtx.Models.User.FindOneByMobileHash(l.ctx, mobileHash.String); err == nil && other.UserId != in.Uid {
				return nil, errMobileUsed
			}
		}
	}
	if in.Email != "" {
		if cur.EmailHash.Valid && cur.EmailHash.String == l.hashIndex(in.Email) {
			email, emailHash = cur.Email, cur.EmailHash
		} else {
			c, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Email))
			if err != nil {
				return nil, errcode.Internal.WithCause(err)
			}
			email = toNullString(c)
			emailHash = toNullString(l.hashIndex(in.Email))
			if other, err := l.svcCtx.Models.User.FindOneByMobileHash(l.ctx, emailHash.String); err == nil && other.UserId != in.Uid {
				return nil, errEmailUsed
			}
		}
	}

	types := cur.Types
	if len(in.Types) > 0 {
		b, err := json.Marshal(in.Types)
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		types = string(b)
	}
	nickname := cur.Nickname
	if in.Nickname != "" {
		nickname = in.Nickname
	}

	if err := l.svcCtx.Models.User.UpdateProfile(l.ctx, tid, in.Uid, nickname,
		toNullInt64(in.OrgId), toNullInt64(in.PartyId), types,
		mobile, mobileHash, email, emailHash, opUID(l.ctx)); err != nil {
		if isDupKey(err) {
			switch dupKeyName(err) {
			case "uk_user_mobile_hash":
				return nil, errMobileUsed
			case "uk_user_email_hash":
				return nil, errEmailUsed
			}
		}
		return nil, errcode.Internal.WithCause(err)
	}

	// 角色重绑（全删全插）+ auth_cache 刷新（降权即时生效）
	if len(in.RoleIds) > 0 {
		if err := l.svcCtx.Models.UserRole.DeleteByUser(l.ctx, tid, in.Uid); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		if err := l.svcCtx.Models.UserRole.BatchInsert(l.ctx, in.Uid, in.RoleIds, tid, opUID(l.ctx)); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
	}
	if err := ensureAuthCache(l.ctx, l.svcCtx, tid, in.Uid); err != nil {
		l.Errorf("auth_cache 刷新失败 uid=%d: %v", in.Uid, err)
	}
	return &pb.UpdateUserResp{}, nil
}

var _ = model.ErrNotFound

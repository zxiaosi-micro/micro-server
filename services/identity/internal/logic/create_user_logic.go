package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateUser 后台建号：mobile 加密+hash 落库，argon2id 密码，绑定角色。
func (l *CreateUserLogic) CreateUser(in *pb.CreateUserReq) (*pb.CreateUserResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Nickname == "" || in.Mobile == "" {
		return nil, errcode.ErrBadRequest.WithMsg("nickname/mobile 必填")
	}
	if len(in.Password) < 8 {
		return nil, errcode.ErrBadRequest.WithMsg("密码至少 8 位")
	}
	if len(in.Types) == 0 {
		return nil, errcode.ErrBadRequest.WithMsg("至少指定一个可登录端")
	}

	mobileCipher, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Mobile))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	mobileHash := l.hashIndex(in.Mobile)

	var emailCipher, emailHash sql.NullString
	if in.Email != "" {
		c, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Email))
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		emailCipher = toNullString(c)
		emailHash = toNullString(l.hashIndex(in.Email))
	}

	passwordHash, err := crypto.HashPassword(in.Password)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	types, err := json.Marshal(in.Types)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	uid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.User.Insert(l.ctx, &model.User{
		UserId:       uid,
		OrgId:        toNullInt64(in.OrgId),
		Nickname:     in.Nickname,
		Mobile:       toNullString(mobileCipher),
		MobileHash:   toNullString(mobileHash),
		Email:        emailCipher,
		EmailHash:    emailHash,
		PasswordHash: toNullString(passwordHash),
		Types:        string(types),
		Status:       1,
		TenantId:     tid,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    toNullInt64(op),
		UpdatedBy:    toNullInt64(op),
	})
	if err != nil {
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

	// 角色绑定（INSERT IGNORE 幂等）+ auth_cache 预热
	if len(in.RoleIds) > 0 {
		if err := l.svcCtx.Models.UserRole.BatchInsert(l.ctx, uid, in.RoleIds, tid, op); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		if err := ensureAuthCache(l.ctx, l.svcCtx, tid, uid); err != nil {
			l.Errorf("auth_cache 预热失败 uid=%d: %v", uid, err)
		}
	}
	l.Infof("建号 uid=%d tenant=%d by=%d", uid, tid, op)
	return &pb.CreateUserResp{Uid: uid}, nil
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

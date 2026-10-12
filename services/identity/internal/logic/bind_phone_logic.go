package logic

import (
	"context"
	"encoding/json"
	"strings"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type BindPhoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPhoneLogic {
	return &BindPhoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindPhone 绑定手机号（S9-01 客户端小程序 need_bind_phone 流程收尾）：
// 加密 + HMAC 索引落库（uk_user_mobile_hash 查重），types 缺当前端时追加。
func (l *BindPhoneLogic) BindPhone(in *pb.BindPhoneReq) (*pb.BindPhoneResp, error) {
	if in.Uid <= 0 || in.Phone == "" {
		return nil, errcode.ErrBadRequest.WithMsg("uid/phone 必填")
	}

	user, err := l.svcCtx.Models.User.FindOneByUID(l.ctx, in.Uid)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errUserNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}

	// 手机号唯一性（加密列无索引，等值查询走 hash 列）
	hash := hmacHex(l.svcCtx.HashKey, in.Phone)
	dup, err := l.svcCtx.Models.User.FindOneByMobileHash(l.ctx, hash)
	if err != nil && err != model.ErrNotFound {
		return nil, errcode.Internal.WithCause(err)
	}
	if dup != nil && dup.UserId != in.Uid {
		return nil, errMobileUsed
	}

	// types 追加当前端（影子账号首登只有微信端；绑定后保持该端可登录）
	types := mergeClientType(user.Types, in.Client)

	mobileCipher, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Phone))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if err := l.svcCtx.Models.User.BindMobile(l.ctx, user.TenantId, in.Uid, types, mobileCipher, hash, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	auditLogin(l.ctx, l.svcCtx.Conn, in.Uid, user.TenantId, "auth.bind_phone")
	l.Infof("绑定手机号 uid=%d client=%s", in.Uid, in.Client)
	return &pb.BindPhoneResp{}, nil
}

// mergeClientType types JSON 追加 client（已含则原样返回；空串沿用客户端小程序端）。
func mergeClientType(typesJSON, client string) string {
	if client == "" {
		client = ctxkit.ClientClientMini
	}
	var list []string
	_ = json.Unmarshal([]byte(typesJSON), &list)
	for _, t := range list {
		if strings.EqualFold(t, client) {
			return typesJSON
		}
	}
	list = append(list, client)
	out, err := json.Marshal(list)
	if err != nil {
		return typesJSON
	}
	return string(out)
}

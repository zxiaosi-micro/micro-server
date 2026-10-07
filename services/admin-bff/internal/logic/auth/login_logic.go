package auth

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login 密码登录：转发 identity（client 固定 ADMIN_WEB；互斥踢旧/锁定/审计都在 identity）。
func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	resp, err := l.svcCtx.Identity.LoginByPassword(l.ctx, &pb.LoginByPasswordReq{
		Mobile:   req.Mobile,
		Password: req.Password,
		Client:   ctxkitClient,
	})
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{
		Tokens:   tokenPair(resp.Tokens),
		UID:      strconv.FormatInt(resp.Uid, 10),
		Nickname: resp.Nickname,
	}, nil
}

// ctxkitClient admin-bff 固定登录端（02 §9.5 client 匹配）。
const ctxkitClient = "ADMIN_WEB"

func tokenPair(t *pb.TokenPair) types.TokenPair {
	if t == nil {
		return types.TokenPair{}
	}
	return types.TokenPair{
		AccessToken:      t.AccessToken,
		RefreshToken:     t.RefreshToken,
		AccessExpiresIn:  t.AccessExpiresIn,
		RefreshExpiresIn: t.RefreshExpiresIn,
	}
}

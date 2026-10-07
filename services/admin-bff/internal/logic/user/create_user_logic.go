package user

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateUserLogic) CreateUser(req *types.UserCreateReq) (*types.SimpleResp, error) {
	resp, err := l.svcCtx.Identity.CreateUser(l.ctx, &pb.CreateUserReq{
		Nickname: req.Nickname,
		Mobile:   req.Mobile,
		Email:    req.Email,
		Password: req.Password,
		Types:    req.Types,
		OrgId:    parseID(req.OrgID),
		RoleIds:  parseIDs(req.RoleIDs),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(resp.Uid, 10)}, nil
}

// —— string ID ↔ int64（E8：对外一律字符串） ——

func parseID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseIDs(list []string) []int64 {
	out := make([]int64, 0, len(list))
	for _, s := range list {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

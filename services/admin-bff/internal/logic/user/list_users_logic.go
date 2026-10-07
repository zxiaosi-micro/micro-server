package user

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListUsers 用户列表（列=姓名/手机号脱敏/部门/端标签/状态）。
func (l *ListUsersLogic) ListUsers(req *types.UserListReq) (*types.UserListResp, error) {
	resp, err := l.svcCtx.Identity.ListUsers(l.ctx, &pb.ListUsersReq{
		Keyword: req.Keyword,
		Status:  int32(req.Status),
		Page:    int64(req.Page),
		Size:    int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	list := make([]types.UserItem, 0, len(resp.List))
	for _, u := range resp.List {
		list = append(list, userItem(u))
	}
	return &types.UserListResp{List: list, Total: resp.Total}, nil
}

// userItem pb → types（ID 一律字符串，E8）。
func userItem(u *pb.UserItem) types.UserItem {
	item := types.UserItem{
		UID:          strconv.FormatInt(u.Uid, 10),
		Nickname:     u.Nickname,
		MobileMasked: u.MobileMasked,
		EmailMasked:  u.EmailMasked,
		Types:        u.Types,
		Status:       int(u.Status),
		RoleCodes:    u.RoleCodes,
	}
	if u.OrgId > 0 {
		item.OrgID = strconv.FormatInt(u.OrgId, 10)
	}
	if u.PartyId > 0 {
		item.PartyID = strconv.FormatInt(u.PartyId, 10)
	}
	if u.LockedUntil > 0 {
		item.LockedUntil = strconv.FormatInt(u.LockedUntil, 10)
	}
	if u.CreatedAt > 0 {
		item.CreatedAt = strconv.FormatInt(u.CreatedAt, 10)
	}
	if item.Types == nil {
		item.Types = []string{}
	}
	if item.RoleCodes == nil {
		item.RoleCodes = []string{}
	}
	return item
}

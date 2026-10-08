// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContactsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 联系人列表(perm: party:party:list)
func NewListContactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContactsLogic {
	return &ListContactsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListContactsLogic) ListContacts(req *types.IDPath) (resp *types.ContactListResp, err error) {
	r, err := l.svcCtx.Party.ListContact(l.ctx, &partyPb.ListContactReq{PartyId: parseID(req.ID)})
	if err != nil {
		return nil, err
	}
	resp = &types.ContactListResp{}
	for _, c := range r.List {
		resp.List = append(resp.List, types.ContactItem{
			ContactID: strconv.FormatInt(c.ContactId, 10), PartyID: strconv.FormatInt(c.PartyId, 10),
			Name: c.Name, Mobile: c.Mobile, Position: c.Position,
			IsDefault: c.IsDefault, NotifyPref: c.NotifyPref, CreatedAt: c.CreatedAt,
		})
	}
	return resp, nil
}

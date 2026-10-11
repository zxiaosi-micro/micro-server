// contacts_logic.go · 我的联系人（party_id 隔离；mobile 脱敏展示）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	ppb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
)

type ContactsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContactsLogic {
	return &ContactsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ContactsLogic) Contacts(req *types.NoArgsReq) (resp *types.ContactListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Party.ListContact(l.ctx, &ppb.ListContactReq{PartyId: partyId})
	if err != nil {
		return nil, err
	}
	resp = &types.ContactListResp{List: []types.ContactView{}}
	for _, c := range out.List {
		resp.List = append(resp.List, types.ContactView{
			ContactId: i64s(c.ContactId),
			Name:      c.Name,
			Mobile:    crypto.MaskPhone(c.Mobile),
			Position:  c.Position,
			IsDefault: c.IsDefault,
		})
	}
	return resp, nil
}

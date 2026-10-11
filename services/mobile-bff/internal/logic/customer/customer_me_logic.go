// customer_me_logic.go · 我的档案（uid → party；requireParty 入口示例）。

package customer

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	ppb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CustomerMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerMeLogic {
	return &CustomerMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CustomerMeLogic) CustomerMe(req *types.NoArgsReq) (resp *types.CustomerMeResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out, err := l.svcCtx.Identity.GetUser(l.ctx, &ipb.GetUserReq{Uid: uid})
	if err != nil {
		return nil, err
	}
	resp = &types.CustomerMeResp{
		UID:          i64s(uid),
		Nickname:     out.GetUser().Nickname,
		MobileMasked: out.GetUser().MobileMasked,
		HasParty:     out.GetUser().PartyId > 0,
	}
	if out.GetUser().PartyId > 0 {
		p, err := l.svcCtx.Party.GetParty(l.ctx, &ppb.GetPartyReq{PartyId: out.GetUser().PartyId})
		if err == nil && p.GetParty() != nil {
			resp.Party = types.PartyView{
				PartyId: i64s(p.Party.PartyId),
				Name:    p.Party.Name,
				Types:   jsonStrings(p.Party.Type),
				Status:  "NORMAL",
			}
			if p.Party.Status == 2 {
				resp.Party.Status = "DISABLED"
			}
		}
	}
	return resp, nil
}

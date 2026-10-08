package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type AddContactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddContactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddContactLogic {
	return &AddContactLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// AddContact 新增联系人：mobile AES-256-GCM 加密 + HMAC 索引哈希（同 identity user.mobile 口径）；
// is_default=1 时清旧默认并插入同事务。
func (l *AddContactLogic) AddContact(in *pb.AddContactReq) (*pb.AddContactResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.PartyId <= 0 || in.Name == "" || in.Mobile == "" {
		return nil, errcode.ErrBadRequest.WithMsg("party_id/name/mobile 必填")
	}
	if !validJSONString(in.NotifyPref) {
		return nil, errcode.ErrBadRequest.WithMsg("notify_pref 不是合法 JSON")
	}
	if _, err := l.svcCtx.Models.Party.FindOne(l.ctx, tid, in.PartyId); err != nil {
		return nil, partyErr(err)
	}

	mobileCipher, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Mobile))
	if err != nil {
		return nil, errcode.Internal.WithMsg("手机号加密失败").WithCause(err)
	}
	mobileHash := hmacHex(l.svcCtx.HashKey, in.Mobile)

	cid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	def := int64(0)
	if in.IsDefault {
		def = 1
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if in.IsDefault {
			if err := l.svcCtx.Models.Contact.ClearDefault(ctx, session, tid, in.PartyId, op); err != nil {
				return err
			}
		}
		return insertContact(ctx, session, l.svcCtx, tid, op, in.PartyId,
			in.Name, mobileCipher, mobileHash, in.Position, def, in.NotifyPref, cid)
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.AddContactResp{ContactId: cid}, nil
}

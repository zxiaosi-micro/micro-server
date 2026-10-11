// helpers.go · customer 包共用：requireParty（FR-PTY-009）与类型转换。

package customer

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"

	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

func i64s(n int64) string { return strconv.FormatInt(n, 10) }

// parseI64 端侧 string ID → int64（非法=0，下游 ErrNotFound 语义统一）。
func parseI64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// errPartyRequired 账号未绑定参与方（requireParty 拒绝；平台段 110404）。
var errPartyRequired = errcode.New(errcode.SegPlatform, 404, "账号未绑定参与方，请先完成客户档案建立")

// requireParty uid → identity.GetUser → party_id。
// S9-01 客户端小程序一切查询按 party_id 隔离的统一入口；未绑定/无效会话即拒。
func requireParty(ctx context.Context, sc *svc.ServiceContext) (int64, error) {
	uid := ctxkit.UID(ctx)
	if uid == 0 {
		return 0, errcode.ErrTokenInvalid
	}
	out, err := sc.Identity.GetUser(ctx, &ipb.GetUserReq{Uid: uid})
	if err != nil {
		return 0, err
	}
	if out.GetUser().PartyId == 0 {
		return 0, errPartyRequired
	}
	return out.GetUser().PartyId, nil
}

// jsonStrings JSON 数组串 → []string（空串安全）。
func jsonStrings(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}

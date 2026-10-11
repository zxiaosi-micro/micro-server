// helpers.go · auth 包共用转换。

package auth

import (
	"strconv"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/types"
)

func i64s(n int64) string { return strconv.FormatInt(n, 10) }

func tokenPair(t *ipb.TokenPair) types.TokenPair {
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

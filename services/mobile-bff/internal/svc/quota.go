package svc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
	"github.com/zxiaosi-micro/micro-common/sessionx"
	"github.com/zxiaosi-micro/micro-common/tenantx"
	ipb "micro-server/services/identity/pb"
)

// Quota 租户 RPM 配额限流（S11-01，02 §9.4；admin-bff 同款镜像）。
//
// 数据面：
//   - 配额源：identity ListTenants（60s TTL 本地缓存）；quota JSON 约定 {"rpm": N}，
//     0/缺省/default 租户不限（tenantx.QuotaLimiter 语义）；
//   - 请求面租户解析：Bearer → sid/uid → auth_cache 快照 TenantID（sid 级 60s 本地缓存，
//     独立于 authz 的每请求 GET）；快照 Miss/验签失败 = 免鉴权或异常请求，不限放行；
//   - 判定：tenantx.QuotaLimiter（Redis 分钟桶）；超限 WriteRateLimited 短路 429（code 110429）；
//     Redis 故障 Allow 内部 fail-open 放行，错误日志即告警信号。
func Quota(l *tenantx.QuotaLimiter, verifier *jwtauth.Verifier, sessions *sessionx.Store) rest.Middleware {
	sidTenant := &sidTenantCache{store: sessions}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if tid, ok := sidTenant.tenantOf(r.Context(), r.Header.Get("Authorization"), verifier); ok {
				d, err := l.Allow(r.Context(), tid)
				if err != nil {
					// fail-open 已在 Allow 内体现（Allowed=true），此处只留告警日志
					logx.Errorf("[quota] redis 故障 fail-open: tenant=%d err=%v", tid, err)
				}
				if !d.Allowed {
					tenantx.WriteRateLimited(w, d)
					return
				}
			}
			next(w, r)
		}
	}
}

// sidTenantCache sid → tenant 解析缓存（60s；会话期内租户不变，消掉每请求 auth_cache GET）。
type sidTenantCache struct {
	mu    sync.RWMutex
	m     map[string]sidTenantEntry
	store *sessionx.Store
}
type sidTenantEntry struct {
	tenantID int64
	until    time.Time
}

func (c *sidTenantCache) tenantOf(ctx context.Context, auth string, verifier *jwtauth.Verifier) (int64, bool) {
	if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") {
		return 0, false
	}
	claims, err := verifier.Verify(strings.TrimSpace(auth[7:]))
	if err != nil || claims.SID == "" || claims.UID <= 0 {
		return 0, false
	}
	c.mu.RLock()
	if e, ok := c.m[claims.SID]; ok && time.Now().Before(e.until) {
		c.mu.RUnlock()
		return e.tenantID, e.tenantID > 0
	}
	c.mu.RUnlock()

	tid := int64(0)
	if snap, err := c.store.GetAuthCache(ctx, claims.UID); err == nil {
		tid = snap.TenantID
	} // Miss/Redis 故障：视为解析失败（不限），60s 内不重试该 sid
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]sidTenantEntry{}
	}
	c.m[claims.SID] = sidTenantEntry{tenantID: tid, until: time.Now().Add(60 * time.Second)}
	c.mu.Unlock()
	return tid, tid > 0
}

// NewQuotaLimiter 组装租户配额器：Redis 沿用会话中心实例（独立 key 前缀），
// quota 源走 identity ListTenants（S11-01；quota JSON 键约定 {"rpm": N}）。
func NewQuotaLimiter(addr, password string, identity ipb.IdentityClient) *tenantx.QuotaLimiter {
	rdb, err := redis.NewRedis(redis.RedisConf{Host: addr, Pass: password, Type: "node"})
	if err != nil {
		panic(err)
	}
	src := &quotaSource{identity: identity}
	l, err := tenantx.NewQuotaLimiter(rdb, "micro:mobile-bff:quota:", src.rpmOf)
	if err != nil {
		panic(err)
	}
	return l
}

// quotaSource 租户 RPM 快照（60s TTL；拉取失败 30s 内不再重试，fail-open 不限）。
type quotaSource struct {
	mu       sync.Mutex
	rpm      map[int64]int
	until    time.Time
	identity ipb.IdentityClient
}

func (s *quotaSource) rpmOf(ctx context.Context, tenantID int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.until) {
		return s.rpm[tenantID]
	}
	m := map[int64]int{}
	// ListTenants 分页上限 100（租户数过百后改分页拉全）
	resp, err := s.identity.ListTenants(ctx, &ipb.ListTenantsReq{Page: 1, Size: 100})
	if err != nil {
		s.until = time.Now().Add(30 * time.Second) // identity 不可达：短退避 + 不限
		return 0
	}
	for _, t := range resp.List {
		m[t.TenantId] = parseQuotaRPM(t.Quota)
	}
	s.rpm = m
	s.until = time.Now().Add(60 * time.Second)
	return m[tenantID]
}

// parseQuotaRPM 解析 quota JSON 的 rpm 键（{"rpm":120}）；非法/缺失 = 0 不限。
func parseQuotaRPM(raw string) int {
	if raw == "" {
		return 0
	}
	var v struct {
		RPM int `json:"rpm"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return 0
	}
	return v.RPM
}

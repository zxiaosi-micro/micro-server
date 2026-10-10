// 影子写入端（Redis；Lua 时间戳比较防乱序，FR-IOT-005 / 02 §9.6）。
package writer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

const shadowLua = `
local cur = redis.call('HGET', KEYS[1], 'ts')
local incoming = ARGV[1]
if cur and tonumber(cur) > tonumber(incoming) then
  return 0
end
redis.call('HSET', KEYS[1], 'ts', incoming)
for i = 2, #ARGV, 2 do
  redis.call('HSET', KEYS[1], ARGV[i], ARGV[i+1])
end
redis.call('EXPIRE', KEYS[1], 86400)
return 1`

// RedisShadow Redis 影子（writer 侧唯一写入方；键与 device 服务 shadow 包对齐）。
type RedisShadow struct{ rd *redis.Redis }

// NewShadowStore 构造（实现 ShadowStore 接口）。
func NewShadowStore(rd *redis.Redis) ShadowStore { return &RedisShadow{rd: rd} }

// Write 原子乱序防护写入（ts 回退直接丢弃）。
func (s *RedisShadow) Write(ctx context.Context, sn string, ts int64, metrics map[string]float64, raw string) error {
	args := []any{strconv.FormatInt(ts, 10), "raw", raw}
	for k, v := range metrics {
		args = append(args, k, strconv.FormatFloat(v, 'f', -1, 64))
	}
	res, err := s.rd.EvalCtx(ctx, shadowLua, []string{"micro:iot:shadow:" + sn}, args...)
	if err != nil {
		return err
	}
	if n, _ := res.(int64); n == 0 {
		return fmt.Errorf("stale ts dropped sn=%s", sn) // 乱序丢弃（预期语义）
	}
	return nil
}

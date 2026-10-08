// Package cronx · 定时任务注册表模板（S3-06 示例；02 §9.6 纪律）。
//
// 新服务照抄本文件：
//   - 定时任务实现 service.Service（go-zero ServiceGroup 挂载），Stop 优雅退出；
//   - **每个任务必须在 register() 登记**（名称/间隔/职责），并维护 lastRun——
//     "cron 注册表"是跨进程延迟兜底扫描的对账依据（ADR-09：内存态定时只做加速，
//     持久状态（next_exec_at 扫描）才是兜底，重启不失）；
//   - 任务体禁止 panic（recover + ERROR 日志）；间隔秒级起步，慢任务跳帧（in-flight 防堆积）。
package cronx

import (
	"context"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

// Task 注册表条目。
type Task struct {
	// Name 任务唯一名（监控/last_run 指标标签）。
	Name string
	// Interval 执行间隔（秒级起步）。
	Interval time.Duration
	// Run 任务体（返回错误仅记日志，不打断后续轮次）。
	Run func(ctx context.Context) error
}

type runner struct {
	task    Task
	lastRun time.Time
}

var (
	mu       sync.Mutex
	registry = map[string]*runner{}
)

// Register 登记任务（重复登记同 panic——启动期暴露配置冲突）。
func Register(t Task) {
	if t.Name == "" || t.Interval <= 0 || t.Run == nil {
		panic("cronx: 任务登记缺少 name/interval/run")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[t.Name]; ok {
		panic("cronx: 任务重复登记 " + t.Name)
	}
	registry[t.Name] = &runner{task: t}
}

// Registrations 返回注册表快照（/metrics 或排障接口对账用）。
func Registrations() []Task {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Task, 0, len(registry))
	for _, r := range registry {
		out = append(out, r.task)
	}
	return out
}

// LastRun 查任务最近执行时间（0 = 未执行过）。
func LastRun(name string) time.Time {
	mu.Lock()
	defer mu.Unlock()
	if r, ok := registry[name]; ok {
		return r.lastRun
	}
	return time.Time{}
}

// NewService 把全部已登记任务变成 service.Service（main 里 ServiceGroup.Add）。
func NewService() service.Service {
	return &cronService{}
}

type cronService struct {
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}
}

func (s *cronService) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.stopped = make(chan struct{})
	mu.Lock()
	runners := make([]*runner, 0, len(registry))
	for _, r := range registry {
		runners = append(runners, r)
	}
	mu.Unlock()

	for _, r := range runners {
		go s.loop(r)
	}
	logx.Infof("cronx: %d 个定时任务已启动", len(runners))
}

func (s *cronService) loop(r *runner) {
	s.runOnce(r) // 启动即跑第一轮（短命进程也能看到 lastRun）
	ticker := time.NewTicker(r.task.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			close(s.stopped)
			return
		case <-ticker.C:
			s.runOnce(r)
		}
	}
}

func (s *cronService) runOnce(r *runner) {
	defer func() {
		if p := recover(); p != nil {
			logx.Errorf("cronx: 任务 %s panic: %v", r.task.Name, p)
		}
	}()
	mu.Lock()
	r.lastRun = time.Now()
	mu.Unlock()
	if err := r.task.Run(s.ctx); err != nil {
		logx.Errorf("cronx: 任务 %s 失败: %v", r.task.Name, err)
	}
}

func (s *cronService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.stopped != nil {
		<-s.stopped
	}
	logx.Info("cronx: 全部定时任务已停止")
}

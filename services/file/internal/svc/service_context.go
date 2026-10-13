package svc

import (
	"context"
	"fmt"
	"os"
	"time"

	"micro-server/services/file/internal/config"
	"micro-server/services/file/internal/model"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext file 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // file_db 主库连接
	Models *Models       // 模型聚合
	// MinIO S3 客户端（启动期幂等建双桶，02 §6.5）。
	Minio      *minio.Client
	Snowflake  *snowflake.Node // 文件 ID 生成器
	snowCancel func()          // worker_id 租约释放（停机时调用）
}

// Models model 聚合。
type Models struct {
	FileMeta model.FileMetaModel // 文件元数据（ETag/桶/对象键）
}

// NewServiceContext 组装根：雪花 ID/MinIO 客户端硬依赖（失败即 panic），双桶启动期自愈建桶。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	mcli, err := minio.New(c.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.Minio.AccessKey, c.Minio.SecretKey, ""),
		Secure: c.Minio.UseSSL,
	})
	if err != nil {
		panic(fmt.Errorf("minio 客户端构造失败: %w", err))
	}

	// 双桶幂等创建（compose init 容器兜底之外的服务级自愈）
	for _, b := range []string{c.Minio.BucketDefault, c.Minio.BucketContract} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		exists, aerr := mcli.BucketExists(ctx, b)
		if aerr == nil && !exists {
			if merr := mcli.MakeBucket(ctx, b, minio.MakeBucketOptions{}); merr != nil {
				logx.Errorf("file: 建桶 %s 失败（首次上传自愈重试）: %v", b, merr)
			} else {
				logx.Infof("file: 已创建桶 %s", b)
			}
		}
		cancel()
	}

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Models:     &Models{FileMeta: model.NewFileMetaModel(conn, c.Cache)},
		Minio:      mcli,
		Snowflake:  node,
		snowCancel: snowCancel,
	}

	logx.Infof("file 就绪：db=file_db minio=%s buckets=%s,%s", c.Minio.Endpoint, c.Minio.BucketDefault, c.Minio.BucketContract)
	return sc
}

// SignKey 下载令牌 HMAC 密钥（env 注入；dev 缺省键）。
func (sc *ServiceContext) SignKey() []byte {
	if v := os.Getenv(sc.Config.SignKeyEnv); v != "" {
		return []byte(v)
	}
	return []byte("micro-dev-file-sign-key")
}

// DefaultTokenExp 令牌缺省有效期。
func (sc *ServiceContext) DefaultTokenExp() time.Duration {
	sec := sc.Config.DefaultTokenExpSec
	if sec <= 0 {
		sec = 300
	}
	return time.Duration(sec) * time.Second
}

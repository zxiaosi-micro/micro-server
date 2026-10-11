// tenantx 登记（analytics_db；tenantaudit CI 对账依据）。
// 读模型/台账各表查询显式带 tenant_id 条件（ADR-08 口径），
// 未在此登记的既有 analytics 表按 S8 落地口径由消费侧信封恢复租户。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	// 数仓星型模型（S10-02）：平台级聚合面，tenant_id 是分析维度而非隔离条件
	// （BI 服务 admin 管理域跨租户分析；ETL INSERT..SELECT 平台级直读业务库）。
	tenantx.Exempt("dw_dim_date", "数仓日历维:无租户概念(S10-02 星型模型)")
	tenantx.Exempt("dw_dim_tenant", "数仓租户维:tenant_id 即主键维度(S10-02 星型模型)")
	tenantx.Exempt("dw_dim_party", "数仓参与方维:平台级快照,tenant_id 为维度非隔离条件(S10-02)")
	tenantx.Exempt("dw_fact_order", "数仓订单事实:平台级聚合,tenant_id 为维度非隔离条件(S10-02)")
	tenantx.Exempt("dw_fact_payment", "数仓支付事实:平台级聚合,tenant_id 为维度非隔离条件(S10-02)")
	tenantx.Exempt("dw_etl_log", "数仓 ETL 批次台账:调度对账平台级(S10-02)")
}

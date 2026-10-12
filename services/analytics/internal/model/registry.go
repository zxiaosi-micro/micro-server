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

	// S11-01 多租户复核补登记：读模型/导出/对账业务表（000001）为租户表，
	// 查询显式带 tenant_id 条件（ADR-08）；水位/元数据/游标为库级平台表。
	tenantx.Scoped("report_event_entry")
	tenantx.Scoped("report_sales_daily")
	tenantx.Scoped("report_inventory_daily")
	tenantx.Scoped("report_warranty_daily")
	tenantx.Scoped("report_sla_daily")
	tenantx.Scoped("report_finance_daily")
	tenantx.Scoped("report_station_daily")
	tenantx.Scoped("export_task")
	tenantx.Scoped("recon_diff")
	tenantx.Exempt("report_watermark", "读模型消费水位:库级单行游标,无租户维度(S8)")
	tenantx.Exempt("report_meta", "读模型元数据/版本:库级单行,重建按库推进(S8)")
	tenantx.Exempt("recon_state", "对账游标:平台级批次推进(S8)")
	tenantx.Exempt("report_metric_dict", "指标字典:tenant_id 0=平台级共享行,字典面平台维护(S8)")
}

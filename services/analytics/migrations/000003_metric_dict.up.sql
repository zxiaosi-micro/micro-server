-- analytics_db 000003：指标字典种子（S8-02，FR-ANA-002「口径字典化 18+ 指标定义可查」）。
-- 字典是口径正本：前端报表页表头/单位/说明、对账指标编码、导出列名都以 metric_code 对齐。
-- 固定 ID（9.0e18 段 9000000000000000801+，重复执行幂等 INSERT IGNORE）。

CREATE TABLE IF NOT EXISTS report_metric_dict (
  id           BIGINT UNSIGNED NOT NULL,
  metric_code  VARCHAR(64)  NOT NULL COMMENT '指标编码（聚合列名/对账编码/导出列名同源）',
  name         VARCHAR(64)  NOT NULL COMMENT '指标名',
  report_type  VARCHAR(32)  NOT NULL COMMENT '所属报表 sales/inventory/warranty/sla/finance/station',
  unit         VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '单位（count/yuan/pcs）',
  caliber      VARCHAR(512) NOT NULL COMMENT '口径定义',
  source_event VARCHAR(255) NOT NULL COMMENT '来源事件（点号类型，多个逗号分隔）',
  recon        TINYINT      NOT NULL DEFAULT 1 COMMENT '是否参与日级对账（状态边沿事件口径不参与）',
  sort         INT          NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用/0 停用',
  tenant_id    BIGINT       NOT NULL DEFAULT 0 COMMENT '0=平台级字典',
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_metric_code` (`metric_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='指标字典（口径正本）';

INSERT IGNORE INTO `report_metric_dict`
    (`id`,`metric_code`,`name`,`report_type`,`unit`,`caliber`,`source_event`,`recon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    -- 销售日报（8）
    (9000000000000000801,'orders_created','新增订单数','sales','count','业务日内 order.created 事件数（order_no 业务键幂等）','order.created',1,1,1,0,NOW()),
    (9000000000000000802,'orders_paid','支付完成订单数','sales','count','业务日内 order.paid 事件数（order_no 幂等）','order.paid',1,2,1,0,NOW()),
    (9000000000000000803,'orders_cancelled','取消订单数','sales','count','业务日内 order.cancelled 事件数（含支付超时自动取消）','order.cancelled',1,3,1,0,NOW()),
    (9000000000000000804,'orders_pay_timeout','支付超时数','sales','count','业务日内 order.pay_timeout 事件数（pay_expire_at 扫描触发）','order.pay_timeout',1,4,1,0,NOW()),
    (9000000000000000805,'returns_approved','退货审批通过数','sales','count','业务日内 order.return_approved 事件数','order.return_approved',1,5,1,0,NOW()),
    (9000000000000000806,'shipments_signed','签收发运单数','sales','count','业务日内 shipment.signed 事件数（shipment_no 幂等）','shipment.signed',1,6,1,0,NOW()),
    (9000000000000000807,'devices_shipped','签收设备数','sales','pcs','shipment.signed 事件 payload.sns 去重计数','shipment.signed',1,7,1,0,NOW()),
    (9000000000000000808,'gmv_paid','实付 GMV','sales','yuan','业务日内 order.paid 事件 paid_amount 合计（DECIMAL 精确累加）','order.paid',1,8,1,0,NOW()),
    -- 库存日报（3）
    (9000000000000000811,'stock_in_qty','入库数量','inventory','pcs','业务日内 stock.in 事件 qty 合计（对应 stock_record.biz_type=STOCK_IN）','stock.in',1,1,1,0,NOW()),
    (9000000000000000812,'stock_out_qty','出库数量','inventory','pcs','业务日内 stock.out 事件 qty 合计（对应 stock_record.biz_type=DEDUCT）','stock.out',1,2,1,0,NOW()),
    (9000000000000000813,'low_stock_events','低库存触发次数','inventory','count','stock.low 边沿触发计数（状态事件，biz_key=event_id，不参与对账）','stock.low',0,3,1,0,NOW()),
    -- 质保日报（4）
    (9000000000000000821,'warranties_started','质保起算数','warranty','count','业务日内 warranty.started 事件数（warranty_no 幂等）','warranty.started',1,1,1,0,NOW()),
    (9000000000000000822,'warranties_device','设备质保起算数','warranty','count','warranty.started 且 payload.level=DEVICE','warranty.started',1,2,1,0,NOW()),
    (9000000000000000823,'warranties_station','场站质保起算数','warranty','count','warranty.started 且 payload.level=STATION','warranty.started',1,3,1,0,NOW()),
    (9000000000000000824,'devices_activated','设备激活数','warranty','count','业务日内 device.activated 事件数（sn 幂等）','device.activated',1,4,1,0,NOW()),
    -- SLA 日报（4）
    (9000000000000000831,'alerts_total','告警产生数','sla','count','业务日内 alert.created 事件数（alert_id 幂等；按场站维度分行）','alert.created',1,1,1,0,NOW()),
    (9000000000000000832,'alerts_p1','P1 告警数','sla','count','alert.created 且 payload.level=P1','alert.created',1,2,1,0,NOW()),
    (9000000000000000833,'alerts_p2','P2 告警数','sla','count','alert.created 且 payload.level=P2','alert.created',1,3,1,0,NOW()),
    (9000000000000000834,'alerts_p3','P3 告警数','sla','count','alert.created 且 payload.level=P3','alert.created',1,4,1,0,NOW()),
    -- 财务日报（4）
    (9000000000000000841,'payments_count','支付完成笔数','finance','count','业务日内 order.paid 事件数（按渠道维度分行）','order.paid',1,1,1,0,NOW()),
    (9000000000000000842,'paid_amount','实付金额','finance','yuan','order.paid paid_amount 合计（按渠道维度分行）','order.paid',1,2,1,0,NOW()),
    (9000000000000000843,'refunds_count','退款笔数','finance','count','业务日内 payment.refunded 事件数（refund_no 幂等）','payment.refunded',1,3,1,0,NOW()),
    (9000000000000000844,'refund_amount','退款金额','finance','yuan','payment.refunded amount 合计（按渠道维度分行）','payment.refunded',1,4,1,0,NOW()),
    -- 场站日报（2）
    (9000000000000000851,'stations_created','新建场站数','station','count','业务日内 station.created 事件数（station_no 幂等；按场站维度分行）','station.created',1,1,1,0,NOW()),
    (9000000000000000852,'station_alerts','场站告警数','station','count','业务日内 alert.created 事件数（按场站维度分行）','alert.created',1,2,1,0,NOW());

-- 平台管理员可见（字典为平台级数据，无 role_menu 绑定需求）。

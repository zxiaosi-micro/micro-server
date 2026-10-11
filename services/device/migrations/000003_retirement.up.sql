-- device_db S10-01 梯次利用 + 回收（退役评估）
-- 退役评估（评级判定）/设备 SOH 快照（评估支撑数据）/回收单（C 级报废走回收，监管溯源）。

-- 退役评估（一设备多轮评估历史；一轮评估一条，评级 A/B/C 定稿口径 v4/03 阶段 10）。
-- 评级规则（定稿）：
--   A：SOH >= 80 且循环 < 1500 且一致性达标 → 二次销售给新质保（RETIRED→IN_STOCK 再销售）
--   B：70 <= SOH < 80 → 降容备电，不给销售质保（终态 RETIRED）
--   C：SOH < 70 或安全检测不过 → 报废走回收单（RETIRED→SCRAPPED）
CREATE TABLE IF NOT EXISTS `retirement_assessment`
(
    `assessment_id`        BIGINT         NOT NULL COMMENT '雪花 ID',
    `assessment_no`        VARCHAR(64)    NOT NULL COMMENT '评估单号（RA+日期+雪花尾段）',
    `device_id`            BIGINT         NOT NULL COMMENT '设备',
    `sn`                   VARCHAR(64)    NOT NULL COMMENT 'SN 冗余（列表免联查）',
    `round`                INT            NOT NULL DEFAULT 1 COMMENT '评估轮次（一设备多轮，逐轮 +1）',
    `soh`                  DECIMAL(6, 2)  NOT NULL DEFAULT 0 COMMENT 'SOH 百分比（健康度）',
    `cycle_count`          INT            NOT NULL DEFAULT 0 COMMENT '循环数',
    `resistance_mohm`      DECIMAL(10, 3) NULL COMMENT '内阻 mΩ',
    `consistency_ok`       TINYINT(1)     NOT NULL DEFAULT 1 COMMENT '一致性是否达标（压差/温差阈值）',
    `safety_json`          JSON           NULL COMMENT '安全检测项 JSON（过充/短路/绝缘/热失控等）',
    `grade`                VARCHAR(8)     NULL COMMENT '评级 A/B/C（检测提交时判定；NULL=待检测录入）',
    `disposition`          VARCHAR(16)    NOT NULL DEFAULT 'PENDING' COMMENT '处置状态 PENDING/RESALE/DERATE/RECYCLED',
    `inspection_ticket_no` VARCHAR(64)    NULL COMMENT '检测工单号（复用 ops 体系录入，S10-01）',
    `remark`               VARCHAR(512)   NULL COMMENT '备注',
    `tenant_id`            BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`           DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`           BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`           DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `updated_by`           BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`           DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`assessment_id`),
    UNIQUE KEY `uk_ra_tenant_no` (`tenant_id`, `assessment_no`),
    KEY `idx_ra_device` (`device_id`, `created_at`),
    KEY `idx_ra_tenant_status` (`tenant_id`, `disposition`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='退役评估（梯次利用评级）';

-- 设备 SOH 快照（评估检测数据留痕：SOH/循环/内阻/一致性/温度极值，评估数据正本）。
CREATE TABLE IF NOT EXISTS `device_soh_snapshot`
(
    `snapshot_id`     BIGINT         NOT NULL COMMENT '雪花 ID',
    `assessment_id`   BIGINT         NOT NULL COMMENT '所属评估',
    `device_id`       BIGINT         NOT NULL COMMENT '设备',
    `sn`              VARCHAR(64)    NOT NULL COMMENT 'SN 冗余',
    `soh`             DECIMAL(6, 2)  NOT NULL DEFAULT 0 COMMENT 'SOH 百分比',
    `cycle_count`     INT            NOT NULL DEFAULT 0 COMMENT '循环数',
    `resistance_mohm` DECIMAL(10, 3) NULL COMMENT '内阻 mΩ',
    `min_cell_v_mv`   INT            NULL COMMENT '单体最低电压 mV',
    `max_cell_v_mv`   INT            NULL COMMENT '单体最高电压 mV',
    `max_temp_dc`     DECIMAL(5, 1)  NULL COMMENT '最高温度 ℃（十分位）',
    `measured_at`     DATETIME(3)    NOT NULL COMMENT '检测时刻',
    `source`          VARCHAR(16)    NOT NULL DEFAULT 'MANUAL' COMMENT '检测来源 MANUAL/TELEMETRY/BENCH',
    `tenant_id`       BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`snapshot_id`),
    KEY `idx_soh_assessment` (`assessment_id`),
    KEY `idx_soh_device` (`device_id`, `measured_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='设备 SOH 快照（评估支撑数据）';

-- 回收单（C 级报废：回收商/残值/凭证 file/监管溯源码；溯源码全局唯一一次生成）。
CREATE TABLE IF NOT EXISTS `recycle_order`
(
    `recycle_id`     BIGINT         NOT NULL COMMENT '雪花 ID',
    `recycle_no`     VARCHAR(64)    NOT NULL COMMENT '回收单号（RC+日期+雪花尾段）',
    `assessment_id`  BIGINT         NOT NULL COMMENT '来源退役评估',
    `device_id`      BIGINT         NOT NULL COMMENT '设备',
    `sn`             VARCHAR(64)    NOT NULL COMMENT 'SN 冗余',
    `recycler_party_id` BIGINT      NULL COMMENT '回收商参与方（party RECYCLER 类型）',
    `residual_value` DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '残值（回收商报价/结算金额）',
    `trace_code`     VARCHAR(64)    NOT NULL COMMENT '监管溯源码（全局唯一，回收留痕）',
    `evidence_file_id` BIGINT       NULL COMMENT '凭证文件（file 服务；交接单/称重单/照片归档）',
    `status`         VARCHAR(16)    NOT NULL DEFAULT 'CREATED' COMMENT 'CREATED/TRANSFERRED/SETTLED',
    `remark`         VARCHAR(512)   NULL COMMENT '备注',
    `tenant_id`      BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `updated_by`     BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`recycle_id`),
    UNIQUE KEY `uk_ro_tenant_no` (`tenant_id`, `recycle_no`),
    UNIQUE KEY `uk_ro_trace_code` (`trace_code`),
    KEY `idx_ro_assessment` (`assessment_id`),
    KEY `idx_ro_device` (`device_id`),
    KEY `idx_ro_tenant_status` (`tenant_id`, `status`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='回收单（C 级报废溯源）';

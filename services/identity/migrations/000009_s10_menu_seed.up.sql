-- S10 增值域菜单集（S10-01~03 /auth/menus 数据源 + 前端页面路由源）。
-- 结构对齐 02 §6.2：目录(1)/菜单(2)/按钮(3)。固定 ID（9.0e18 段，S10 起用 9000000000000000811+）；
-- 重复执行幂等（INSERT IGNORE）；role_menu 同步绑定平台管理员角色（role_code=admin，seed 种入）。
-- perm_code 与 admin-bff perm_resolver / 前端 <Perm code> 严格同源。

INSERT IGNORE INTO `menu`
    (`menu_id`,`parent_id`,`name`,`type`,`perm_code`,`path`,`icon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    -- 资产中心 · 退役评估（S10-01 梯次利用 + 回收）
    (9000000000000000811, 9000000000000000601, '退役评估', 2, 'asset:device:retire', '/asset/retirements', 'recycle', 5, 1, 9000000000000000001, NOW()),
    -- 分析中心 · BI 自助 + 数仓调度（S10-02）
    (9000000000000000821, 9000000000000000701, 'BI 自助', 2, 'analytics:bi:query', '/analytics/bi', 'fund-view', 3, 1, 9000000000000000001, NOW()),
    (9000000000000000822, 9000000000000000821, '数仓调度', 3, 'analytics:recon:run', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    -- 财务中心 · 经销商返利（S10-03）
    (9000000000000000831, 9000000000000000431, '返利结算', 2, 'finance:rebate:list', '/finance/rebates', 'gift', 5, 1, 9000000000000000001, NOW()),
    (9000000000000000832, 9000000000000000831, '月度结算', 3, 'finance:rebate:settle', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000833, 9000000000000000831, '结算确认', 3, 'finance:rebate:confirm', NULL, NULL, 2, 1, 9000000000000000001, NOW());

-- 平台管理员角色绑定全部新菜单（role_code=admin，seed 9000000000000000003）。
INSERT IGNORE INTO `role_menu`
    (`role_id`,`menu_id`,`tenant_id`,`created_at`)
SELECT 9000000000000000003, m.`menu_id`, 9000000000000000001, NOW()
FROM `menu` m
WHERE m.`menu_id` >= 9000000000000000811 AND m.`menu_id` <= 9000000000000000833
  AND m.`tenant_id` = 9000000000000000001;

-- S8 分析域菜单集（S8-05 /auth/menus 数据源 + 前端页面路由源）。
-- 结构对齐 02 §6.2：目录(1)/菜单(2)/按钮(3)。固定 ID（9.0e18 段，S8 起用 9000000000000000701+）；
-- 重复执行幂等（INSERT IGNORE）；role_menu 同步绑定平台管理员角色（role_code=admin，seed 种入）。
-- perm_code 与 admin-bff perm_resolver / 前端 <Perm code> 严格同源。

INSERT IGNORE INTO `menu`
    (`menu_id`,`parent_id`,`name`,`type`,`perm_code`,`path`,`icon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    -- 分析中心目录
    (9000000000000000701, 0, '分析中心', 1, NULL, '/analytics', 'bar-chart', 9, 1, 9000000000000000001, NOW()),
    (9000000000000000711, 9000000000000000701, '报表中心', 2, 'analytics:report:list', '/analytics/reports', 'line-chart', 1, 1, 9000000000000000001, NOW()),
    (9000000000000000712, 9000000000000000711, '手动对账', 3, 'analytics:recon:run', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000713, 9000000000000000711, '读模型重建', 3, 'analytics:recon:rebuild', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000721, 9000000000000000701, '下载中心', 2, 'analytics:export:list', '/analytics/download-center', 'download', 2, 1, 9000000000000000001, NOW()),
    (9000000000000000722, 9000000000000000721, '发起导出', 3, 'analytics:export:create', NULL, NULL, 1, 1, 9000000000000000001, NOW());

-- 平台管理员角色绑定全部新菜单（role_code=admin，seed 9000000000000000003）。
INSERT IGNORE INTO `role_menu`
    (`role_id`,`menu_id`,`tenant_id`,`created_at`)
SELECT 9000000000000000003, m.`menu_id`, 9000000000000000001, NOW()
FROM `menu` m
WHERE m.`menu_id` >= 9000000000000000701 AND m.`menu_id` <= 9000000000000000721
  AND m.`tenant_id` = 9000000000000000001;

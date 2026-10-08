-- S5 交易域菜单集（S5-02~04 /auth/menus 数据源 + 前端页面路由源）。
-- 结构对齐 02 §6.2：目录(1)/菜单(2)/按钮(3)。固定 ID（9.0e18 段，S5 起用 9000000000000000401+）；
-- 重复执行幂等（INSERT IGNORE）；role_menu 同步绑定平台管理员角色（role_code=admin，seed 种入）。
-- perm_code 与 admin-bff perm_resolver / 前端 <Perm code> 严格同源。

INSERT IGNORE INTO `menu`
    (`menu_id`,`parent_id`,`name`,`type`,`perm_code`,`path`,`icon`,`sort`,`status`,`tenant_id`,`created_at`)
VALUES
    -- 交易中心目录
    (9000000000000000401, 0, '交易中心', 1, NULL, '/trade', 'shopping-cart', 5, 1, 9000000000000000001, NOW()),
    (9000000000000000411, 9000000000000000401, '订单管理', 2, 'trade:order:list', '/trade/orders', 'profile', 1, 1, 9000000000000000001, NOW()),
    (9000000000000000412, 9000000000000000411, '创建订单', 3, 'trade:order:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000413, 9000000000000000411, '订单支付', 3, 'trade:order:pay', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000414, 9000000000000000411, '取消订单', 3, 'trade:order:cancel', NULL, NULL, 3, 1, 9000000000000000001, NOW()),
    (9000000000000000415, 9000000000000000411, 'Saga 人工重推', 3, 'trade:order:retry', NULL, NULL, 4, 1, 9000000000000000001, NOW()),
    (9000000000000000416, 9000000000000000411, '发货单创建', 3, 'trade:shipment:create', NULL, NULL, 5, 1, 9000000000000000001, NOW()),
    (9000000000000000417, 9000000000000000411, '轨迹录入', 3, 'trade:shipment:trace', NULL, NULL, 6, 1, 9000000000000000001, NOW()),
    (9000000000000000418, 9000000000000000411, '签收确认', 3, 'trade:shipment:sign', NULL, NULL, 7, 1, 9000000000000000001, NOW()),
    (9000000000000000421, 9000000000000000401, '退货管理', 2, 'trade:order:list', '/trade/returns', 'rollback', 2, 1, 9000000000000000001, NOW()),
    (9000000000000000422, 9000000000000000421, '退货申请', 3, 'trade:return:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000423, 9000000000000000421, '退货审批', 3, 'trade:return:approve', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    -- 财务中心目录
    (9000000000000000431, 0, '财务中心', 1, NULL, '/finance', 'money-collect', 7, 1, 9000000000000000001, NOW()),
    (9000000000000000441, 9000000000000000431, '支付单', 2, 'finance:payment:list', '/finance/payments', 'pay-circle', 1, 1, 9000000000000000001, NOW()),
    (9000000000000000442, 9000000000000000441, '支付确认', 3, 'finance:payment:confirm', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000443, 9000000000000000441, '对公复核', 3, 'finance:payment:approve', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000444, 9000000000000000441, '对账核销', 3, 'finance:payment:settle', NULL, NULL, 3, 1, 9000000000000000001, NOW()),
    (9000000000000000451, 9000000000000000431, '退款单', 2, 'finance:refund:list', '/finance/refunds', 'wallet', 2, 1, 9000000000000000001, NOW()),
    (9000000000000000452, 9000000000000000451, '手工退款', 3, 'finance:refund:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000461, 9000000000000000431, '发票', 2, 'finance:invoice:list', '/finance/invoices', 'red-envelope', 3, 1, 9000000000000000001, NOW()),
    (9000000000000000462, 9000000000000000461, '开票', 3, 'finance:invoice:issue', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000463, 9000000000000000461, '红冲', 3, 'finance:invoice:reverse', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000471, 9000000000000000431, '对账任务', 2, 'finance:reconcile:list', '/finance/reconcile-tasks', 'swap', 4, 1, 9000000000000000001, NOW()),
    (9000000000000000472, 9000000000000000471, '对账处理', 3, 'finance:reconcile:resolve', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    -- 合同质保目录
    (9000000000000000481, 0, '合同质保', 1, NULL, '/contract', 'file-protect', 8, 1, 9000000000000000001, NOW()),
    (9000000000000000491, 9000000000000000481, '合同管理', 2, 'contract:contract:list', '/contract/contracts', 'file-done', 1, 1, 9000000000000000001, NOW()),
    (9000000000000000492, 9000000000000000491, '归档上传', 3, 'contract:contract:archive', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000501, 9000000000000000481, '质保查询', 2, 'contract:warranty:list', '/contract/warranties', 'safety-certificate', 2, 1, 9000000000000000001, NOW()),
    (9000000000000000511, 9000000000000000481, 'SLA 策略', 2, 'contract:sla:list', '/contract/sla', 'clock-circle', 3, 1, 9000000000000000001, NOW()),
    (9000000000000000512, 9000000000000000511, '策略创建', 3, 'contract:sla:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000513, 9000000000000000511, '合同绑定', 3, 'contract:sla:bind', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000521, 9000000000000000481, '索赔管理', 2, 'contract:claim:list', '/contract/claims', 'exception', 4, 1, 9000000000000000001, NOW()),
    (9000000000000000522, 9000000000000000521, '索赔申请', 3, 'contract:claim:create', NULL, NULL, 1, 1, 9000000000000000001, NOW()),
    (9000000000000000523, 9000000000000000521, '索赔审批', 3, 'contract:claim:approve', NULL, NULL, 2, 1, 9000000000000000001, NOW()),
    (9000000000000000524, 9000000000000000521, '索赔结算', 3, 'contract:claim:settle', NULL, NULL, 3, 1, 9000000000000000001, NOW()),
    (9000000000000000531, 9000000000000000481, '延保销售', 2, 'contract:warranty:list', '/contract/extensions', 'crown', 5, 1, 9000000000000000001, NOW());

-- 平台管理员角色绑定全部新菜单（role_code=admin，seed 9000000000000000003）。
INSERT IGNORE INTO `role_menu`
    (`role_id`,`menu_id`,`tenant_id`,`created_at`)
SELECT 9000000000000000003, m.`menu_id`, 9000000000000000001, NOW()
FROM `menu` m
WHERE m.`menu_id` >= 9000000000000000401 AND m.`menu_id` <= 9000000000000000531
  AND m.`tenant_id` = 9000000000000000001;

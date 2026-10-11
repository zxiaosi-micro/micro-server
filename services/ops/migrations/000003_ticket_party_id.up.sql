-- S9-01 客户端小程序 requireParty：工单补报修客户列（type=AFTER_SALE 由 mobile-bff 写入）。
-- 读侧 ListTicket 增 party_id 过滤；历史行 party_id 为 NULL 视为未关联客户。
ALTER TABLE `ticket`
    ADD COLUMN `party_id` BIGINT NULL COMMENT '报修客户（type=AFTER_SALE，S9-01 requireParty 隔离）' AFTER `client_action_id`,
    ADD KEY `idx_ticket_party` (`tenant_id`, `party_id`, `status`);

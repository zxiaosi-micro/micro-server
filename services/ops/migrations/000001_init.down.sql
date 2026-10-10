-- ops_db 回滚（S7-01）：按依赖逆序删除。
DROP TABLE IF EXISTS `inspection_item`;
DROP TABLE IF EXISTS `inspection_task`;
DROP TABLE IF EXISTS `inspection_plan`;
DROP TABLE IF EXISTS `inspection_template`;
DROP TABLE IF EXISTS `sla_timer`;
DROP TABLE IF EXISTS `ticket_log`;
DROP TABLE IF EXISTS `ticket`;
DROP TABLE IF EXISTS `alert`;
DROP TABLE IF EXISTS `alert_rule`;
DROP TABLE IF EXISTS `event_outbox`;

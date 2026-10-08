-- finance_db 000001 回滚：按建表逆序 drop。
DROP TABLE IF EXISTS `event_outbox`;
DROP TABLE IF EXISTS `rebate_settlement`;
DROP TABLE IF EXISTS `reconcile_task`;
DROP TABLE IF EXISTS `invoice`;
DROP TABLE IF EXISTS `refund`;
DROP TABLE IF EXISTS `payment`;

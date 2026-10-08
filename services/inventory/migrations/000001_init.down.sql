-- inventory_db 000001 回滚：逆序 DROP。
DROP TABLE IF EXISTS `event_outbox`;
DROP TABLE IF EXISTS `stocktake_item`;
DROP TABLE IF EXISTS `stocktake`;
DROP TABLE IF EXISTS `stock_record`;
DROP TABLE IF EXISTS `inventory`;
DROP TABLE IF EXISTS `warehouse`;

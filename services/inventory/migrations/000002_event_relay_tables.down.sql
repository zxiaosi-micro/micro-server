-- inventory Outbox Relay 接线配套回滚。
DROP TABLE IF EXISTS event_dead;
DROP TABLE IF EXISTS event_retry;

DROP TRIGGER IF EXISTS trg_protect_account_currency ON accounts;
DROP FUNCTION IF EXISTS enforce_account_currency_immutability();

DROP TRIGGER IF EXISTS trg_protect_event_log_immutability ON event_log;
DROP FUNCTION IF EXISTS prevent_event_log_mutation();

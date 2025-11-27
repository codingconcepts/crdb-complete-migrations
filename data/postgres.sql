CREATE DATABASE bank;
USE bank;

CREATE SCHEMA bank_svc;

CREATE TABLE bank_svc.customer (
  id INT DEFAULT unique_rowid() PRIMARY KEY,
  name STRING NOT NULL,
  email STRING UNIQUE NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE bank_svc.account (
  id INT DEFAULT unique_rowid() PRIMARY KEY,
  balance DECIMAL(15, 2) NOT NULL,
  customer_id INT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  CONSTRAINT fk_account_customer
    FOREIGN KEY (customer_id)
    REFERENCES bank_svc.customer(id)
);

CREATE TYPE bank_svc.customer_account_id AS (
  customer_id INT,
  account_id INT
);

CREATE OR REPLACE FUNCTION bank_svc.open_account(
  p_name            STRING,
  p_email           STRING,
  p_initial_balance DECIMAL
) RETURNS bank_svc.customer_account_id
LANGUAGE SQL
VOLATILE
AS $$
  WITH
    ins_customer AS (
      INSERT INTO bank_svc.customer (name, email)
      VALUES (p_name, p_email)
      RETURNING id
    ),
    ins_account AS (
      INSERT INTO bank_svc.account (balance, customer_id)
      SELECT p_initial_balance, id FROM ins_customer
      RETURNING id, customer_id
    )
  SELECT customer_id, id FROM ins_account
$$;

CREATE OR REPLACE PROCEDURE bank_svc.make_transfer(
  p_from_account_id INT,
  p_to_account_id   INT,
  p_amount          DECIMAL
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_from_balance DECIMAL;
  v_to_balance   DECIMAL;
BEGIN
  IF p_amount IS NULL OR p_amount <= 0 THEN
    RAISE EXCEPTION USING
      ERRCODE = '22003',
      MESSAGE = 'Transfer amount must be greater than zero';
  END IF;

  IF p_from_account_id = p_to_account_id THEN
    RAISE EXCEPTION USING
      ERRCODE = '22003',
      MESSAGE = 'Cannot transfer to the same account';
  END IF;

  -- Check that source account exists (and lock row).
  SELECT balance INTO v_from_balance
  FROM bank_svc.account
  WHERE id = p_from_account_id
  FOR UPDATE;
  
  -- Check that destination account exists (and lock row).
  SELECT balance INTO v_to_balance
  FROM bank_svc.account
  WHERE id = p_to_account_id
  FOR UPDATE;

  SELECT balance
    INTO v_from_balance
    FROM bank_svc.account
   WHERE id = p_from_account_id;

  IF v_from_balance < p_amount THEN
    RAISE EXCEPTION USING
      ERRCODE = '22000',
      MESSAGE = 'Insufficient funds in source account';
  END IF;

  UPDATE bank_svc.account
     SET balance = balance - p_amount
   WHERE id = p_from_account_id;

  UPDATE bank_svc.account
     SET balance = balance + p_amount
   WHERE id = p_to_account_id;

END;
$$;

ALTER TABLE bank_svc.account DROP CONSTRAINT fk_account_customer;

CREATE DATABASE _replicator;

ALTER DATABASE _replicator CONFIGURE ZONE USING gc.ttlseconds=300;
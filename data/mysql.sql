CREATE USER 'bank_svc'@'%' IDENTIFIED BY 'password';

CREATE DATABASE bank;
USE bank;

CREATE TABLE customer (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL
);

CREATE TABLE account (
  id INT AUTO_INCREMENT PRIMARY KEY,
  balance DECIMAL(15, 2) NOT NULL,
  customer_id INT NOT NULL,
  CONSTRAINT fk_account_customer
    FOREIGN KEY (customer_id)
    REFERENCES customer(id)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON bank.customer TO 'bank_svc'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON bank.account TO 'bank_svc'@'%';
FLUSH PRIVILEGES;

DELIMITER $$
CREATE PROCEDURE open_account(
  IN p_name VARCHAR(255),
  IN p_email VARCHAR(255),
  IN p_initial_balance DECIMAL(15, 2),
  OUT v_customer_id INT,
  OUT v_account_id INT
)
BEGIN
  INSERT INTO customer (name, email)
  VALUES (p_name, p_email);
  
  SET v_customer_id = LAST_INSERT_ID();
  
  INSERT INTO account (balance, customer_id)
  VALUES (p_initial_balance, v_customer_id);
  
  SET v_account_id = LAST_INSERT_ID();
END$$
DELIMITER ;


DELIMITER $$
CREATE PROCEDURE make_transfer(
  IN p_from_account_id INT,
  IN p_to_account_id INT,
  IN p_amount DECIMAL(15, 2)
)
MODIFIES SQL DATA
BEGIN
  DECLARE v_from_balance DECIMAL(15, 2);
  DECLARE v_to_balance DECIMAL(15, 2);
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    ROLLBACK;
    RESIGNAL;
  END;
  
  START TRANSACTION;
  
  IF p_amount IS NULL OR p_amount <= 0 THEN
    SIGNAL SQLSTATE '22003'
      SET MESSAGE_TEXT = 'Transfer amount must be greater than zero';
  END IF;

  IF p_from_account_id = p_to_account_id THEN
    SIGNAL SQLSTATE '22003'
      SET MESSAGE_TEXT = 'Cannot transfer to the same account';
  END IF;

  SELECT balance INTO v_from_balance
  FROM account
  WHERE id = p_from_account_id
  FOR UPDATE;
  
  IF v_from_balance IS NULL THEN
    SIGNAL SQLSTATE '22000'
      SET MESSAGE_TEXT = 'Source account does not exist';
  END IF;
  
  SELECT balance INTO v_to_balance
  FROM account
  WHERE id = p_to_account_id
  FOR UPDATE;
  
  IF v_to_balance IS NULL THEN
    SIGNAL SQLSTATE '22000'
      SET MESSAGE_TEXT = 'Destination account does not exist';
  END IF;

  IF v_from_balance < p_amount THEN
    SIGNAL SQLSTATE '22000'
      SET MESSAGE_TEXT = 'Insufficient funds in source account';
  END IF;

  UPDATE account
  SET balance = balance - p_amount
  WHERE id = p_from_account_id;

  UPDATE account
  SET balance = balance + p_amount
  WHERE id = p_to_account_id;
  
  COMMIT;
END$$
DELIMITER ;

-------------
-- Helpers --
-------------

DELIMITER $$
CREATE PROCEDURE populate_test_data()
BEGIN
  DECLARE i INT DEFAULT 1;
  DECLARE v_account_id INT;
  DECLARE v_name VARCHAR(100);
  DECLARE v_email VARCHAR(100);
  DECLARE v_balance DECIMAL(15, 2);
  
  WHILE i <= 1000 DO
    SET v_name = CONCAT('customer ', i);
    SET v_email = CONCAT('customer', i, '@example.com');
    SET v_balance = ROUND(10000 + (RAND() * 90000), 2);
    
    SET v_account_id = open_account(v_name, v_email, v_balance);
    
    SET i = i + 1;
  END WHILE;
  
  COMMIT;
END$$
DELIMITER ;

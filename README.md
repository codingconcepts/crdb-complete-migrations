# crdb-complete-migrations
A collection of complete database migrations from a variety of source databases to CockroachDB.

### Resources

* [CockroachDB to MySQL](https://github.com/cockroachdb/replicator/wiki/C2MySQL#cockroachdb-to-mysqlmariadb)
* [CockroachDB to Oracle](https://github.com/cockroachdb/molt/wiki/Replicator:-C2Oracle-example)

### Terminals



### Introduction

* What we'll be doing today
  * Installing our first database
  * Exploring the application code
  * Starting a workload
  * Preparing for migration

### Dependencies

Build MOLT fetch and MOLT replicator

```sh
cd github.com/cockroachdb/molt
CGO_ENABLED=1 go build -v -ldflags="-s -w" -tags target_oracle -o .

cd github.com/cockroachdb/replicator
CGO_ENABLED=1 go build -v -ldflags="-s -w" -tags target_oracle -o .
```

Install Oracle Client

### Setup (Oracle)

Database

```sh
docker run \
-d \
--name oracle \
-p 1521:1521 \
-p 5500:5500 \
-e ORACLE_PDB=defaultdb \
-e ORACLE_PWD=password \
container-registry.oracle.com/database/enterprise:19.19.0.0

docker logs oracle -f
```

Create Oracle objects and populate accounts

```sh
echo exit | sqlplus system/password@"localhost:1521/defaultdb" @data/oracle.sql
```

### Setup (MySQL)

Database

``` sh
docker run -d \
  --name mysql \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=password \
    mysql:9.5.0 \
      --server-id=1 \
      --log-bin=mysql-bin \
      --binlog-format=ROW \
      --binlog_row_metadata=FULL \
      --gtid-mode=ON \
      --enforce-gtid-consistency \
      --log-slave-updates
```

Connect to MySQL

```sh
mysql -h 127.0.0.1 -P 3306 -u root -p
```

**Create objects** - from data/mysql.sql

Create test accounts

```sql
CALL populate_test_data();
```

Run source API

```sh
go run api/api.go \
--url "root:password@tcp(localhost:3306)/defaultdb" \
--driver mysql \
--addr "localhost:3001"
```

### Migration (Oracle)

Run API

```sh
go run api/api.go \
--url "oracle://system:password@localhost:1521/defaultdb" \
--driver oracle \
--addr "localhost:3000"
```

Workload

```sh
qapi \
--config client/qapi.yaml \
--vus 10 \
--duration 10s
```

Start CockroachDB

```sh
docker run -d \
  --name cockroach \
  -p 26257:26257 \
  -p 8080:8080 \
    cockroachdb/cockroach:v25.3.4 \
      start-single-node \
      --store=path=/cockroach/cockroach-data,size=640MiB \
      --insecure
```

Create objects

```sh
cockroach sql --insecure -f data/postgres.sql
```

**Create objects** - from data/postgres.sql

### Migration (MySQL)

Run MOLT Fetch

```sh
molt fetch \
--source "mysql://root:password@localhost:3306/bank" \
--target "postgres://root@localhost:26257/bank?sslmode=disable" \
--direct-copy \
--allow-tls-mode-disable \
--logging trace
```

Get the GTID state (assuming connection to `mysql -h 127.0.0.1 -P 3306 -u root -p`)

```sql
SELECT @@GLOBAL.gtid_executed;
```

Run MOLT Replicator

```sh
replicator mylogical \
--sourceConn "mysql://root:password@localhost:3306/bank?sslmode=disable" \
--targetConn "postgres://root@localhost:26257/bank?sslmode=disable" \
--targetSchema bank.public \
--defaultGTIDSet "a9b60378-c60c-11f0-a72c-0242ac110002:1-1495" \
-v
```

Run target API

```sh
go run api/api.go \
--url "postgres://root@localhost:26257/bank?sslmode=disable" \
--driver pgx \
--addr "localhost:3002"
```

Drain the load balancer

```sh
curl -s http://localhost:28687/ports/3000/activate \
--json '{
  "groups": ["app_source", "app_target"],
  "weights": [0, 0]
}'
```

Once requests cease, switch to the target api

```sh
curl -s http://localhost:28687/ports/3000/activate \
--json '{
  "groups": ["app_source", "app_target"],
  "weights": [0, 100]
}'
```

Calculate checksums

```sql
-- MySQL
SELECT SHA2(GROUP_CONCAT(
  SHA2(CONCAT_WS('|', id, customer_id, balance), 256)
  ORDER BY id
), 256) AS table_hash
FROM bank.account;

-- CockroachDB
SELECT SHA256(STRING_AGG(
  SHA256(CONCAT_WS('|', id, customer_id, balance)), 
  '' ORDER BY id
)) AS table_hash
FROM bank.account;
```

### Migration (Oracle)

Hop into container

```sh
docker exec -it oracle bash

sqlplus / as sysdba
```

Run the following commands

```sql
--- Enable archive log
SELECT log_mode FROM v$database;
SHUTDOWN IMMEDIATE;
STARTUP MOUNT;
ALTER DATABASE ARCHIVELOG;
ALTER DATABASE OPEN;
SELECT log_mode FROM v$database;

-- Enable suplimental PK logging
ALTER DATABASE ADD SUPPLEMENTAL LOG DATA (PRIMARY KEY) COLUMNS;
SELECT supplemental_log_data_min, supplemental_log_data_pk FROM v$database;

ALTER DATABASE FORCE LOGGING;

-- Create common user.
CREATE USER C##MIGRATION_USER IDENTIFIED BY "password";

-- TESTING
GRANT CONNECT TO C##MIGRATION_USER;
GRANT CREATE SESSION TO C##MIGRATION_USER;

-- General metadata access
GRANT EXECUTE_CATALOG_ROLE TO C##MIGRATION_USER;
GRANT SELECT_CATALOG_ROLE TO C##MIGRATION_USER;

-- Access to necessary V$ views
GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;

-- Direct grants to specific DBA views
GRANT SELECT ON ALL_USERS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_USERS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_OBJECTS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_SYNONYMS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_TABLES TO C##MIGRATION_USER;

-- Switch to PDB
ALTER SESSION SET CONTAINER = defaultdb;
SHOW CON_NAME;

GRANT CONNECT TO C##MIGRATION_USER;
GRANT CREATE SESSION TO C##MIGRATION_USER;

-- General metadata access
GRANT SELECT_CATALOG_ROLE TO C##MIGRATION_USER;

-- Access to necessary V$ views
GRANT SELECT ON V_$SESSION TO C##MIGRATION_USER;
GRANT SELECT ON V_$TRANSACTION TO C##MIGRATION_USER;

-- Grant these two for every table to migrate in the migration_schema
GRANT SELECT, FLASHBACK ON bank_svc.customer TO C##MIGRATION_USER;
GRANT SELECT, FLASHBACK ON bank_svc.account TO C##MIGRATION_USER;

-- Configure source database for replication
CREATE TABLE bank_svc."REPLICATOR_SENTINEL" (
  keycol NUMBER PRIMARY KEY,
  lastSCN NUMBER
);

GRANT SELECT, INSERT, UPDATE ON bank_svc."REPLICATOR_SENTINEL" TO C##MIGRATION_USER;

GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGFILE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_CONTENTS TO C##MIGRATION_USER;
GRANT SELECT ON V_$ARCHIVED_LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG_HISTORY TO C##MIGRATION_USER;
GRANT SELECT ON V_$THREAD TO C##MIGRATION_USER;
GRANT SELECT ON V_$PARAMETER TO C##MIGRATION_USER;
GRANT SELECT ON V_$TIMEZONE_NAMES TO C##MIGRATION_USER;
GRANT SELECT ON V_$INSTANCE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_STATS TO C##MIGRATION_USER;

-- SYS-prefixed views (for full dictionary access)
GRANT SELECT ON SYS.V_$LOGMNR_DICTIONARY TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_LOGS TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_PARAMETERS TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_SESSION TO C##MIGRATION_USER;

-- Access to LogMiner views and controls
GRANT LOGMINING TO C##MIGRATION_USER;
GRANT EXECUTE_CATALOG_ROLE TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR_D TO C##MIGRATION_USER;

ALTER SESSION SET CONTAINER = CDB$ROOT;

-- Grant access to the common user
GRANT EXECUTE ON DBMS_LOGMNR TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR_D TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGFILE TO C##MIGRATION_USER;
GRANT SELECT ON V_$ARCHIVED_LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;
GRANT SELECT ON V_$THREAD TO C##MIGRATION_USER;
GRANT SELECT ON V_$PARAMETER TO C##MIGRATION_USER;
GRANT SELECT ON V_$TIMEZONE_NAMES TO C##MIGRATION_USER;
GRANT SELECT ON V_$INSTANCE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_LOGS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_CONTENTS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_PARAMETERS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_STATS TO C##MIGRATION_USER;

-- Also needed: access to redo logs
GRANT SELECT ON V_$LOG_HISTORY TO C##MIGRATION_USER;

GRANT LOGMINING TO C##MIGRATION_USER;

-- Check
SELECT
  l.GROUP#,
  lf.MEMBER,
  l.FIRST_CHANGE# AS START_SCN,
  l.NEXT_CHANGE# AS END_SCN
FROM V$LOG l
JOIN V$LOGFILE lf
ON l.GROUP# = lf.GROUP#;
```

Configure LogMiner

```sql
ALTER SESSION SET CONTAINER = defaultdb;

-- Get the current snapshot System Change Number
SELECT CURRENT_SCN FROM V$DATABASE;

-- Apply the system change number (replacing the SCN accordingly)
EXEC DBMS_LOGMNR.START_LOGMNR(STARTSCN => 1184040, OPTIONS  => DBMS_LOGMNR.DICT_FROM_ONLINE_CATALOG);
```

### Migration (Oracle)

Connect to CockroachDB

```sh
cockroach sql --insecure --database bank
```

Show no data in CockroachDB

```sql
SELECT COUNT(*) FROM bank_svc.account;
```

Run migration from Oracle to CockroachDB

```sh
# Export env vars
export LD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3
export DYLD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3

molt fetch \
--source "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--source-cdb "oracle://c%23%23migration_user:password@localhost:1521/ORCLCDB" \
--target "postgres://root@localhost:26257/bank?sslmode=disable" \
--mode data-load \
--schema-filter bank_svc \
--direct-copy \
--allow-tls-mode-disable \
--compression none \
--local-path data/molt \
--log-file stdout
```

Show data in CockroachDB

```sql
SELECT COUNT(*) FROM bank_svc.account;
```

Capture the `--scn` and `--backfilFromSCN` values from the output

Run replicator to stream incremental changes

```sh
# Export env vars
export LD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3
export DYLD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3

replicator oraclelogminer \
--sourceConn "oracle://c%23%23migration_user:password@localhost:1521/ORCLCDB" \
--sourcePDBConn "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--sourceSchema BANK_SVC \
--targetConn "postgres://root@localhost:26257/bank?sslmode=disable" \
--targetSchema bank.bank_svc \
--backfillFromSCN 1185661 --scn 1185662 \
-v
```

Observe data arriving into CockroachDB from Oracle

```sql
SELECT MAX(updated_at) FROM bank_svc.account;
```

**Start of cutover**

Stop API

Run MOLT Verify

```sh
# Export env vars
export LD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3
export DYLD_LIBRARY_PATH=/Users/$USER/dev/bin/instantclient_23_3

molt verify \
--source "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--source-cdb "oracle://c%23%23migration_user:password@localhost:1521/ORCLCDB" \
--target "postgres://root@localhost:26257/bank?sslmode=disable" \
--allow-tls-mode-disable \
--log-file stdout \
| grep -v 'warn' \
| grep '"type":"summary"' \
| jq
```

Stop running replicator

Create updated_at triggers in CockroachDB

```sql
CREATE OR REPLACE FUNCTION bank_svc.customer_before_update()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE PLpgSQL;


CREATE OR REPLACE FUNCTION bank_svc.account_before_update()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE PLpgSQL;


CREATE TRIGGER customer_before_update
BEFORE INSERT OR UPDATE ON bank_svc.customer
FOR EACH ROW
EXECUTE FUNCTION bank_svc.customer_before_update();

CREATE TRIGGER account_before_update
BEFORE INSERT OR UPDATE ON bank_svc.account
FOR EACH ROW
EXECUTE FUNCTION bank_svc.account_before_update();
```

Run API

```sh
go run api/api.go \
--url "postgres://root@localhost:26257/bank?sslmode=disable" \
--driver pgx \
--addr "localhost:3000"
```

Show data arriving

```sql
SELECT MAX(updated_at) FROM bank_svc.account;
```

Migration complete.

### Replication from CockroachDB to Oracle

Disable triggers in Oracle

```sql
ALTER TRIGGER bank_svc.customer_updated_at_trigger DISABLE;
ALTER TRIGGER bank_svc.account_updated_at_trigger DISABLE;
```

Configure rangefeeds in CockroachDB

```sql
SET CLUSTER SETTING kv.rangefeed.enabled = true;

-- To lower changefeed emission latency, but increase SQL foreground latency:
SET CLUSTER SETTING kv.rangefeed.closed_timestamp_refresh_interval = '250ms';

-- To lower the closed timestamp lag duration:
SET CLUSTER SETTING kv.closed_timestamp.target_duration = '1s';

-- To improve catchup speeds but increase cluster CPU usage:
SET CLUSTER SETTING kv.rangefeed.concurrent_catchup_iterators = 64;
```

Get the current timestamp in RFC3339 format.

```sql
SELECT cluster_logical_timestamp();
-- 1764235856384158882.0000000000

date -u +"%Y-%m-%dT%H:%M:%SZ"
-- 
```

Create changefeed

```sql
-- CREATE CHANGEFEED FOR TABLE bank_svc.account
-- INTO 'webhook-https://host.docker.internal:30004/defaultdb?insecure_tls_skip_verify=true' 
-- WITH OPTIONS (
--   cursor = '1764235856384158882.0000000000', 
--   initial_scan = 'no', 
--   min_checkpoint_frequency = '250ms', 
--   resolved = '250ms', 
--   updated, 
--   webhook_sink_config = '{"Flush":{"Bytes":1048576,"Frequency":"1s"}}'
-- );

CREATE CHANGEFEED FOR TABLE bank_svc.customer, bank_svc.account
INTO 'webhook-https://host.docker.internal:30004/BANK_SVC?insecure_tls_skip_verify=true' 
WITH OPTIONS (
  cursor = '1764266875464541820.0000000000', 
  initial_scan = 'no', 
  min_checkpoint_frequency = '1s', 
  resolved = '1s', 
  updated, 
  webhook_sink_config = '{"Flush":{"Bytes":1048576,"Frequency":"1s"}}'
);

-- 1127542985805987841
```

Start a separate CockroachDB database for staging changes

```sh
docker run -d \
  --name cockroach-staging \
  -p 26258:26257 \
  -p 8081:8080 \
    cockroachdb/cockroach:v25.3.4 \
      start-single-node \
      --store=path=/cockroach/cockroach-data,size=640MiB \
      --insecure
```

Start replicator

```sh
replicator start \
--stagingConn "postgres://root@localhost:26257/defaultdb?sslmode=disable" \
--targetConn "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--tlsSelfSigned \
--disableAuthentication \
--bindAddr 0.0.0.0:30004 \
-vv
```

Errors:

* transient error: 400 Bad Request: switcher-defaultdb already registered

Show data arriving into Oracle

```sql
SELECT MAX(updated_at) FROM bank_svc.account;
```

### Summary



### Teardown

Oracle data

```sql
TRUNCATE TABLE bank_svc.account;
TRUNCATE TABLE bank_svc.custome;
DELETE FROM bank_svc.REPLICATOR_SENTINEL;

ALTER TABLE bank_svc.account MODIFY(id GENERATED AS IDENTITY (START WITH 1));
ALTER TABLE bank_svc.customer MODIFY(id GENERATED AS IDENTITY (START WITH 1));
/
```

Oracle objects

```sql
DROP TABLE bank_svc.account;
DROP TABLE bank_svc.customer;
DROP TABLE bank_svc.REPLICATOR_SENTINEL;

DROP PROCEDURE bank_svc.open_account;
DROP PROCEDURE bank_svc.make_transfer;

DROP USER bank_svc CASCADE;
```

CockroachDB data

```sql
TRUNCATE bank_svc.account CASCADE;
TRUNCATE bank_svc.customer CASCADE;

TRUNCATE _molt_fetch_exceptions CASCADE;
TRUNCATE _molt_fetch_status CASCADE;

TRUNCATE _replicator._oracle_checkpoint CASCADE;
TRUNCATE _replicator.leases CASCADE;
TRUNCATE _replicator.memo CASCADE;
```

CockroachDB objects

```sql
DROP FUNCTION bank_svc.open_account;
DROP PROCEDURE bank_svc.make_transfer;

DROP TABLE _molt_fetch_exceptions;
DROP TABLE _molt_fetch_status;
DROP TABLE bank_svc.account;
DROP TABLE bank_svc.customer;

DROP DATABASE _replicator;
```

Everything

```sh
make teardown
```

### Scratchpad

Check that replicator is working

```sql
-- In Oracle.
UPDATE bank_svc.account SET balance = 10000 WHERE id = 1;

-- In CockroachDB.
SELECT balance FROM bank_svc.account WHERE id = 1;
```

View Oracle listeners

```sql
SELECT name FROM v$services;
```

Test source and target APIs (**switch the port**)

```sh
# Test source.
port_no=3001

# Customers.
response_a=$(curl -s http://localhost:${port_no}/api/customers \
--json "{
   \"customer\": {
      \"name\": \"$(gofakeit name)\",
      \"email\": \"$(gofakeit email)\"
   },
   \"initial_balance\": 1000
}")

customer_id_a=$(echo $response_a | jq -r '.customer_id')
account_id_a=$(echo $response_a | jq -r '.account_id')

response_b=$(curl -s http://localhost:${port_no}/api/customers \
--json "{
   \"customer\": {
      \"name\": \"$(gofakeit name)\",
      \"email\": \"$(gofakeit email)\"
   },
   \"initial_balance\": 1000
}")

customer_id_b=$(echo "$response_b" | jq -r '.customer_id')
account_id_b=$(echo "$response_b" | jq -r '.account_id')

curl -s http://localhost:${port_no}/api/customers | jq

curl -s http://localhost:${port_no}/api/customers/${customer_id_a} | jq
curl -s http://localhost:${port_no}/api/customers/${customer_id_b} | jq

# Accounts.
curl -s http://localhost:${port_no}/api/accounts/${account_id_a} | jq
curl -s http://localhost:${port_no}/api/accounts/${account_id_b} | jq

curl -s http://localhost:${port_no}/api/accounts \
--json "{
  \"account_id\": ${account_id_a},
  \"to_account_id\": ${account_id_b},
  \"amount\": 10
}"

curl -s http://localhost:${port_no}/api/accounts/${account_id_a} | jq
curl -s http://localhost:${port_no}/api/accounts/${account_id_b} | jq
```

Seed CockroachDB with test data

```sql
INSERT INTO bank_svc.customer (id, name, email) VALUES
  (1, 'Rob Reid', 'rob@cockroachlabs.com'),
  (2, 'Jane Doe', 'jane.doe@acme.com');

INSERT INTO bank_svc.account (id, balance, customer_id) VALUES
  (1, 1000, 1),
  (2, 10000, 2);
```

Test transfer

```sql
CALL bank_svc.make_transfer(1, 2, 10000);
CALL bank_svc.make_transfer(1, 1, 10);
CALL bank_svc.make_transfer(1, 100000, 10);

CALL bank_svc.make_transfer(1, 2, 10);
SELECT * FROM bank_svc.account;
```

Find customer by their account number

```sql
SELECT a.id, c.*
FROM bank_svc.account a
JOIN bank_svc.customer c ON a.customer_id = c.id
WHERE a.id = 731;
```
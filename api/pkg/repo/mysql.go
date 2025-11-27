package repo

import (
	"complete_migration/api/pkg/model"
	"context"
	"database/sql"
	"fmt"
)

type MySQLRepo struct {
	db *sql.DB
}

func NewMySQLRepo(db *sql.DB) *MySQLRepo {
	return &MySQLRepo{
		db: db,
	}
}

func (r *MySQLRepo) OpenAccount(ctx context.Context, customer model.Customer, initialBalance float64) (int64, int64, error) {
	const stmt = `CALL open_account(?, ?, ?, @customer_id, @account_id)`

	_, err := r.db.ExecContext(ctx, stmt,
		customer.Name,
		customer.Email,
		initialBalance,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("calling procedure: %w", err)
	}

	const selectStmt = `SELECT @customer_id, @account_id`
	var customerID, accountID int64
	err = r.db.QueryRowContext(ctx, selectStmt).Scan(&customerID, &accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("retrieving output parameters: %w", err)
	}

	return customerID, accountID, nil
}

func (r *MySQLRepo) GetCustomer(ctx context.Context, customerID int64) (model.Customer, error) {
	const stmt = `SELECT name, email
								FROM customer
								WHERE id = ?`

	row := r.db.QueryRowContext(ctx, stmt, customerID)

	customer := model.Customer{
		ID: customerID,
	}

	if err := row.Scan(&customer.Name, &customer.Email); err != nil {
		return model.Customer{}, fmt.Errorf("scanning row: %w", err)
	}
	return customer, nil
}

func (r *MySQLRepo) GetCustomers(ctx context.Context) ([]model.Customer, error) {
	const stmt = `SELECT id, name, email
								FROM customer
								LIMIT 100`

	rows, err := r.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("performing query: %w", err)
	}
	defer rows.Close()

	var customers []model.Customer

	for rows.Next() {
		var customer model.Customer
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Email); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		customers = append(customers, customer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return customers, nil
}

func (r *MySQLRepo) GetBalance(ctx context.Context, accountID int64) (float64, error) {
	const stmt = `SELECT balance
								FROM account
								WHERE id = ?`

	row := r.db.QueryRowContext(ctx, stmt, accountID)

	var balance float64

	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("scanning row: %w", err)
	}
	return balance, nil
}

func (r *MySQLRepo) MakeTransfer(ctx context.Context, accountID int64, toAccountID int64, amount float64) error {
	const stmt = `CALL make_transfer(?, ?, ?)`

	_, err := r.db.ExecContext(ctx, stmt,
		accountID,
		toAccountID,
		amount,
	)

	if err != nil {
		return fmt.Errorf("calling procedure: %w", err)
	}

	return nil
}

package repo

import (
	"complete_migration/api/pkg/model"
	"context"
	"database/sql"
	"fmt"
)

type PostgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{
		db: db,
	}
}

func (r *PostgresRepo) OpenAccount(ctx context.Context, customer model.Customer, initialBalance float64) (int64, int64, error) {
	const stmt = `SELECT * FROM bank_svc.open_account($1, $2, $3)`

	var customerID, accountID int64
	row := r.db.QueryRowContext(ctx, stmt,
		customer.Name,
		customer.Email,
		initialBalance,
	)

	if err := row.Scan(&customerID, &accountID); err != nil {
		return 0, 0, fmt.Errorf("calling function: %w", err)
	}

	return customerID, accountID, nil
}

func (r *PostgresRepo) GetCustomer(ctx context.Context, customerID int64) (model.Customer, error) {
	const stmt = `SELECT name, email
								FROM bank_svc.customer
								WHERE id = $1`

	row := r.db.QueryRowContext(ctx, stmt, customerID)

	customer := model.Customer{
		ID: customerID,
	}

	if err := row.Scan(&customer.Name, &customer.Email); err != nil {
		return model.Customer{}, fmt.Errorf("scanning row: %w", err)
	}
	return customer, nil
}

func (r *PostgresRepo) GetCustomers(ctx context.Context) ([]model.Customer, error) {
	const stmt = `SELECT id, name, email
								FROM bank_svc.customer
								LIMIT 100`

	rows, err := r.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("performing query: %w", err)
	}

	var customers []model.Customer
	var customer model.Customer

	for rows.Next() {
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Email); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		customers = append(customers, customer)
	}
	return customers, nil
}

func (r *PostgresRepo) GetBalance(ctx context.Context, accountID int64) (float64, error) {
	const stmt = `SELECT balance
								FROM bank_svc.account
								WHERE id = $1`

	row := r.db.QueryRowContext(ctx, stmt, accountID)

	var balance float64

	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("scanning row: %w", err)
	}
	return balance, nil
}

func (r *PostgresRepo) MakeTransfer(ctx context.Context, accountID int64, toAccountID int64, amount float64) error {
	const stmt = `CALL bank_svc.make_transfer($1, $2, $3)`

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

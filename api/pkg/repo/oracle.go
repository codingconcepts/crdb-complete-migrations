package repo

import (
	"complete_migration/api/pkg/model"
	"context"
	"database/sql"
	"fmt"
)

type OracleRepo struct {
	db *sql.DB
}

func NewOracleRepo(db *sql.DB) *OracleRepo {
	return &OracleRepo{
		db: db,
	}
}

func (r *OracleRepo) OpenAccount(ctx context.Context, customer model.Customer, initialBalance float64) (int64, int64, error) {
	const stmt = `BEGIN
									bank_svc.open_account(
										p_name => :1,
										p_email => :2,
										p_initial_balance => :3,
										p_customer_id => :4,
										p_account_id => :5
									);
								END;`

	var customerID int64
	var accountID int64

	_, err := r.db.ExecContext(ctx, stmt,
		customer.Name,
		customer.Email,
		initialBalance,
		sql.Out{Dest: &customerID},
		sql.Out{Dest: &accountID},
	)

	if err != nil {
		return 0, 0, fmt.Errorf("calling procedure: %w", err)
	}

	return customerID, accountID, nil
}

func (r *OracleRepo) GetCustomer(ctx context.Context, customerID int64) (model.Customer, error) {
	const stmt = `SELECT name, email
								FROM bank_svc.customer
								WHERE id = :1`

	row := r.db.QueryRowContext(ctx, stmt, customerID)

	customer := model.Customer{
		ID: customerID,
	}

	if err := row.Scan(&customer.Name, &customer.Email); err != nil {
		return model.Customer{}, fmt.Errorf("scanning row: %w", err)
	}
	return customer, nil
}

func (r *OracleRepo) GetCustomers(ctx context.Context) ([]model.Customer, error) {
	const stmt = `SELECT id, name, email
								FROM bank_svc.customer
								FETCH FIRST 100 ROWS ONLY`

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

func (r *OracleRepo) GetBalance(ctx context.Context, accountID int64) (float64, error) {
	const stmt = `SELECT balance
								FROM bank_svc.account
								WHERE id = :1`

	row := r.db.QueryRowContext(ctx, stmt, accountID)

	var balance float64

	if err := row.Scan(&balance); err != nil {
		return 0, fmt.Errorf("scanning row: %w", err)
	}
	return balance, nil
}

func (r *OracleRepo) MakeTransfer(ctx context.Context, accountID int64, toAccountID int64, amount float64) error {
	const stmt = `BEGIN
									bank_svc.make_transfer(
										p_from_account_id => :1,
										p_to_account_id => :2,
										p_amount => :3
									);
								END;`

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

package repo

import (
	"complete_migration/api/pkg/model"
	"context"
)

type Repo interface {
	OpenAccount(ctx context.Context, customer model.Customer, initialBalance float64) (int64, int64, error)
	GetBalance(ctx context.Context, accountID int64) (float64, error)
	MakeTransfer(ctx context.Context, accountID int64, toAccountID int64, amount float64) error
	GetCustomer(ctx context.Context, customerID int64) (model.Customer, error)
	GetCustomers(ctx context.Context) ([]model.Customer, error)
}

package handlers

import (
	"complete_migration/api/pkg/repo"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/codingconcepts/errhandler"
)

func GetBalance(repo repo.Repo) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		accountIDRaw := r.PathValue("id")
		if accountIDRaw == "" {
			return errhandler.Error(http.StatusUnprocessableEntity, errors.New("missing id"))
		}

		accountID, err := strconv.ParseInt(accountIDRaw, 10, 64)
		if err != nil {
			return errhandler.Error(http.StatusUnprocessableEntity, err)
		}

		balance, err := repo.GetBalance(r.Context(), accountID)
		if err != nil {
			return errhandler.Error(http.StatusInternalServerError, err)
		}

		return errhandler.SendJSON(w, map[string]any{
			"balance": balance,
		})
	}
}

type makeTransferRequest struct {
	AccountID   int64   `json:"account_id"`
	ToAccountID int64   `json:"to_account_id"`
	Amount      float64 `json:"amount"`
}

func MakeTransfer(repo repo.Repo) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req makeTransferRequest
		err := errhandler.ParseJSON(r, &req)
		if err != nil {
			return errhandler.Error(http.StatusUnprocessableEntity, err)
		}
		defer r.Body.Close()

		if req.Amount == 0 {
			return errhandler.Error(http.StatusBadRequest, fmt.Errorf("transfer amount must be greater than zero"))
		}

		if err = repo.MakeTransfer(r.Context(), req.AccountID, req.ToAccountID, req.Amount); err != nil {
			return errhandler.Error(http.StatusInternalServerError, err)
		}

		return errhandler.SendString(w, "ok")
	}
}

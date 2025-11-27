package handlers

import (
	"complete_migration/api/pkg/model"
	"complete_migration/api/pkg/repo"
	"errors"
	"net/http"
	"strconv"

	"github.com/codingconcepts/errhandler"
)

type openAccountRequest struct {
	Customer       model.Customer `json:"customer"`
	InitialBalance float64        `json:"initial_balance"`
}

func OpenAccount(repo repo.Repo) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req openAccountRequest
		err := errhandler.ParseJSON(r, &req)
		if err != nil {
			return errhandler.Error(http.StatusUnprocessableEntity, err)
		}
		defer r.Body.Close()

		customerID, accountID, err := repo.OpenAccount(r.Context(), req.Customer, req.InitialBalance)
		if err != nil {
			return errhandler.Error(http.StatusInternalServerError, err)
		}

		return errhandler.SendJSON(w, map[string]any{
			"customer_id": customerID,
			"account_id":  accountID,
		})
	}
}

func GetCustomers(repo repo.Repo) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		customers, err := repo.GetCustomers(r.Context())
		if err != nil {
			return errhandler.Error(http.StatusInternalServerError, err)
		}

		return errhandler.SendJSON(w, customers)
	}
}

func GetCustomer(repo repo.Repo) errhandler.Wrap {
	return func(w http.ResponseWriter, r *http.Request) error {
		idRaw := r.PathValue("id")
		if idRaw == "" {
			return errhandler.Error(http.StatusUnprocessableEntity, errors.New("missing id"))
		}

		id, err := strconv.ParseInt(idRaw, 10, 64)
		if err != nil {
			return errhandler.Error(http.StatusUnprocessableEntity, err)
		}

		customer, err := repo.GetCustomer(r.Context(), id)
		if err != nil {
			return errhandler.Error(http.StatusInternalServerError, err)
		}

		return errhandler.SendJSON(w, customer)
	}
}

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// provide all the functions in entries and transactions
type Store struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		Queries: New(db),
		db:      db,
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback error: %w db error: %w", rbErr, err)
		}
		return err
	}
	return tx.Commit()
}

type TransferTxParams struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}
type TransferTxResult struct {
	Transfer            Transfer `json:"transfer"`
	FromAccountDeducted Account  `json:"from_account"`
	ToAccountDeducted   Account  `json:"to_account"`
	FromEntry           Entry    `json:"from_entry"`
	ToEntry             Entry    `json:"to_entry"`
}

var txKey = struct{}{}

func (Store *Store) Transfer(ctx context.Context, args TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult
	err := Store.execTx(ctx, func(q *Queries) error {
		var err error
		// transfer is created
		txName := ctx.Value(txKey)
		fmt.Println(txName, "create transfer")
		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: args.FromAccountID,
			ToAccountID:   args.ToAccountID,
			Amount:        args.Amount,
		})
		if err != nil {
			return err
		}
		fmt.Println(txName, "create entry1")
		// Entry is created
		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: args.FromAccountID,
			Amount:    -args.Amount,
		})
		if err != nil {
			return err
		}
		fmt.Println(txName, "create entry2")
		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: args.ToAccountID,
			Amount:    args.Amount,
		})
		if err != nil {
			return err
		}
		fmt.Println(txName, "create account1")
		account1, err := q.GetAccountForUpdate(ctx, args.FromAccountID)
		if err != nil {
			return err
		}
		fmt.Println(txName, "update account1")
		err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      args.FromAccountID,
			Balance: account1.Balance - args.Amount,
		})
		if err != nil {
			return err
		}
		result.FromAccountDeducted = Account{
			ID:      args.FromAccountID,
			Balance: account1.Balance - args.Amount,
		}
		fmt.Println(txName, "create account2")
		account2, err := q.GetAccountForUpdate(ctx, args.ToAccountID)
		if err != nil {
			return err
		}
		fmt.Println(txName, "update account2")
		err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      args.ToAccountID,
			Balance: account2.Balance + args.Amount,
		})
		if err != nil {
			return err
		}
		result.ToAccountDeducted = Account{
			ID:      args.ToAccountID,
			Balance: account2.Balance + args.Amount,
		}

		return nil
	})
	return result, err
}

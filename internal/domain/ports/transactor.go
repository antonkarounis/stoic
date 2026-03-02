package ports

import "context"

// TxRepositories provides repository instances scoped to a single transaction.
// Services receive this struct inside an InTx callback and must use these
// instances for all writes within that transaction.
type TxRepositories struct {
	Users      UserRepository
}

// Transactor executes a function within a database transaction.
// If fn returns an error the transaction is rolled back; otherwise it is committed.
// The concrete implementation is provided by the DB adapter — the domain layer
// depends only on this interface.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context, repos TxRepositories) error) error
}

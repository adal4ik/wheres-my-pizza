package repository

import "database/sql"

type Repository struct {
	OrderRepo OrderRepositoryInterface
}

func New(db *sql.DB) *Repository {
	return &Repository{
		OrderRepo: NewOrderRepository(db),
	}
}

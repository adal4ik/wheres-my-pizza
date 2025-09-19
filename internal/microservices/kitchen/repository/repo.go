package repository

import "database/sql"

type Repository struct {
	KitchenRepo KitchenRepositoryInterface
}

func New(db *sql.DB) *Repository {
	return &Repository{
		KitchenRepo: NewKitchenRepository(db),
	}
}

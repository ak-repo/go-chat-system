package utils

import "github.com/jackc/pgconn"

func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	pgErr, ok := err.(*pgconn.PgError)
	return ok && pgErr.Code == "23505"
}

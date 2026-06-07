package handler

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)

	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}, nil
}

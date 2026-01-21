package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/vultisig/pluginagent/internal/storage/postgres/queries"
	"github.com/vultisig/pluginagent/internal/types"
)

func toTypesSystemEvent(row queries.SystemEvent) (*types.SystemEvent, error) {
	var policyID *uuid.UUID
	if row.PolicyID.Valid {
		policyIDParsed, err := uuidFromPgUUID(row.PolicyID)
		if err != nil {
			return nil, err
		}
		policyID = &policyIDParsed
	}

	var publicKey *string
	if row.PublicKey.Valid {
		publicKey = &row.PublicKey.String
	}

	return &types.SystemEvent{
		ID:        row.ID,
		PublicKey: publicKey,
		PolicyID:  policyID,
		EventType: types.SystemEventType(row.EventType),
		EventData: row.EventData,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

func uuidToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

func uuidFromPgUUID(pguuid pgtype.UUID) (uuid.UUID, error) {
	if !pguuid.Valid {
		return uuid.Nil, nil
	}
	return pguuid.Bytes, nil
}

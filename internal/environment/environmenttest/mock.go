package environmenttest

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func OnTenantEnvironments(ctx context.Context, expected []*model.TenantEnvironment) {
	rows := make([]environmentsql.TenantEnvironmentsRow, len(expected))

	for i, e := range expected {
		rows[i] = environmentsql.TenantEnvironmentsRow{
			ID:          e.ID,
			Name:        e.Name,
			Ci:          e.CI,
			Description: e.Description,
			Created: pgtype.Timestamptz{
				Time:  e.Created,
				Valid: true,
			},
			LastModified: pgtype.Timestamptz{
				Time:  e.LastModified,
				Valid: true,
			},
			Kind:       environmentsql.EnvironmentKind(e.Kind),
			TenantName: e.TenantName,
			TenantID:   e.TenantID,
		}
	}

	GetQuerier(ctx).EXPECT().TenantEnvironments(mock.Anything, false).Return(rows, nil)
}

func OnGetTenants(ctx context.Context, expected []*model.Tenant) {
	rows := make([]environmentsql.Tenant, len(expected))

	for i, e := range expected {
		rows[i] = environmentsql.Tenant{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Created: pgtype.Timestamptz{
				Time:  e.Created,
				Valid: true,
			},
			LastModified: pgtype.Timestamptz{
				Time:  e.LastModified,
				Valid: true,
			},
			UpgradeDelayDays: e.UpgradeDelayDays,
		}
	}

	GetQuerier(ctx).EXPECT().GetTenants(mock.Anything).Return(rows, nil).Once()
}

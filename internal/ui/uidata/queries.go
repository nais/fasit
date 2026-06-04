package uidata

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
)

func ListTenants(ctx context.Context) ([]*Tenant, error) {
	rows, err := querier(ctx).ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]*Tenant, len(rows))
	for i, tenant := range rows {
		ret[i] = tenantFromSQL(tenant)
	}
	return ret, nil
}

func ListReleaseStatuses(ctx context.Context, environmentID uuid.UUID) ([]*model.Release, error) {
	res, err := querier(ctx).ListReleaseStatuses(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	releases := make([]*model.Release, len(res))
	for i, r := range res {
		releases[i] = &model.Release{
			Name:         r.Feature,
			Version:      r.Version,
			Status:       r.Status,
			Revision:     int(r.Revision),
			LastDeployed: r.LastDeployed.Time,
			Created:      r.Created.Time,
			LastModified: r.LastModified.Time,
		}
	}

	return releases, nil
}

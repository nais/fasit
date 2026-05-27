package uidata

import (
	"context"
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

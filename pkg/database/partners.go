package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

func partnerFromSQL(p gensql.Partner) *model.Partner {
	return &model.Partner{
		ID:           p.ID,
		Name:         p.Name,
		Description:  nullStringToPtr(p.Description),
		Created:      p.Created,
		LastModified: p.LastModified,
	}
}

func (r *repo) PartnerCreate(ctx context.Context, p *model.PartnerCreate) (*model.Partner, error) {
	partner, err := r.querier.PartnerCreate(ctx, gensql.PartnerCreateParams{
		Name:        p.Name,
		Description: ptrToNullString(p.Description),
	})
	if err != nil {
		return nil, err
	}
	return partnerFromSQL(partner), nil
}

func (r *repo) PartnerGet(ctx context.Context, id uuid.UUID) (*model.Partner, error) {
	partner, err := r.querier.PartnerGet(ctx, id)
	if err != nil {
		return nil, err
	}
	return partnerFromSQL(partner), nil
}

func (r *repo) PartnersGet(ctx context.Context) ([]*model.Partner, error) {
	partners, err := r.querier.PartnersGet(ctx)
	if err != nil {
		return nil, err
	}
	partnerSlice := []*model.Partner{}
	for _, partner := range partners {
		partnerSlice = append(partnerSlice, partnerFromSQL(partner))
	}
	return partnerSlice, nil
}

func (r *repo) PartnerEnvironments(ctx context.Context) ([]*model.PartnerEnvironments, error) {
	data, err := r.querier.PartnerEnvironments(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var ret []*model.PartnerEnvironments
	for _, d := range data {
		ret = append(ret, &model.PartnerEnvironments{
			Environment: model.Environment{
				ID:           d.ID,
				Name:         d.Name,
				Description:  nullStringToPtr(d.Description),
				Created:      d.Created,
				LastModified: d.LastModified,
			},
			PartnerName: d.PartnerName,
		})
	}

	return ret, nil
}

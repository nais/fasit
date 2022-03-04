package database

import (
	"context"

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

func (r *Repo) PartnerCreate(ctx context.Context, p *model.PartnerCreate) (*model.Partner, error) {
	partner, err := r.querier.PartnerCreate(ctx, gensql.PartnerCreateParams{
		Name:        p.Name,
		Description: ptrToNullString(p.Description),
	})
	if err != nil {
		return nil, err
	}
	return partnerFromSQL(partner), nil
}

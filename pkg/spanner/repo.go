package spanner

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"github.com/nais/fasit/pkg/graph/model"
)

type Repo struct {
	client *spanner.Client
}

func NewRepo(client *spanner.Client) *Repo {
	return &Repo{
		client: client,
	}
}

func (r *Repo) PartnerCreate(ctx context.Context, p *model.PartnerCreate) (model.ID, error) {
	id := model.NewID()
	_, err := r.client.ReadWriteTransaction(ctx, func(ctx context.Context, rwt *spanner.ReadWriteTransaction) error {
		stmt := spanner.Statement{
			SQL: `INSERT INTO Partners (PartnerID, Name, Description, Created, LastModified) VALUES
				(@id, @name, @description, CURRENT_TIMESTAMP(), CURRENT_TIMESTAMP())`,
			Params: map[string]any{
				"id":          id,
				"name":        p.Name,
				"description": p.Description,
			},
		}
		rowCount, err := rwt.Update(ctx, stmt)
		if err != nil {
			return err
		}
		fmt.Printf("%d record(s) inserted.\n", rowCount)
		return err
	})

	return id, err
}

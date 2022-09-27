package database

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type KubernetesNodeRepo interface {
	KubernetesNodesForEnv(ctx context.Context, envID uuid.UUID) ([]*model.KubernetesNode, error)
	KubernetesNodeSync(ctx context.Context, envID uuid.UUID, kn *message.KubernetesNodes) error
}

func (r *repo) KubernetesNodeSync(ctx context.Context, envID uuid.UUID, kn *message.KubernetesNodes) error {
	for _, n := range kn.Nodes {
		params, err := kubernetesNodeParams(envID, n)
		if err != nil {
			return err
		}
		err = r.querier.KubernetesNodeCreateOrUpdate(ctx, params)
		if err != nil {
			return err
		}
	}
	err := r.querier.KubernetesNodeDeleteObsolete(ctx, envID)
	if err != nil {
		return err
	}
	return nil
}

func (r *repo) KubernetesNodesForEnv(ctx context.Context, envID uuid.UUID) ([]*model.KubernetesNode, error) {
	res, err := r.querier.KubernetesNodeStatuses(ctx, envID)
	if err != nil {
		return nil, err
	}

	nodes := make([]*model.KubernetesNode, len(res))
	for i, r := range res {
		nodes[i], err = kubernetesNodeFromSQL(r)
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func kubernetesNodeParams(envID uuid.UUID, n message.KubernetesNode) (gensql.KubernetesNodeCreateOrUpdateParams, error) {
	conditions, err := json.Marshal(n.Conditions)
	if err != nil {
		return gensql.KubernetesNodeCreateOrUpdateParams{}, err
	}

	allocatable, err := json.Marshal(n.Allocatable)
	if err != nil {
		return gensql.KubernetesNodeCreateOrUpdateParams{}, err
	}

	capacity, err := json.Marshal(n.Capacity)
	if err != nil {
		return gensql.KubernetesNodeCreateOrUpdateParams{}, err
	}

	return gensql.KubernetesNodeCreateOrUpdateParams{
		EnvironmentID:           envID,
		Name:                    n.Name,
		KernelVersion:           n.KernelVersion,
		OsImage:                 n.OSImage,
		ContainerRuntimeVersion: n.ContainerRuntimeVersion,
		KubeletVersion:          n.KubeletVersion,
		KubeProxyVersion:        n.KubeProxyVersion,
		OperatingSystem:         n.OperatingSystem,
		Architecture:            n.Architecture,
		Conditions: pgtype.JSONB{
			Bytes:  conditions,
			Status: pgtype.Present,
		},
		Allocatable: pgtype.JSONB{
			Bytes:  allocatable,
			Status: pgtype.Present,
		},
		Capacity: pgtype.JSONB{
			Bytes:  capacity,
			Status: pgtype.Present,
		},
		InternalIp: n.InternalIP,
	}, nil
}

func kubernetesNodeFromSQL(n gensql.KubernetesNodeStatus) (*model.KubernetesNode, error) {
	conditions := []*model.KubernetesNodeCondition{}
	if err := json.Unmarshal(n.Conditions.Bytes, &conditions); err != nil {
		return nil, err
	}

	allocatable := &model.KubernetesNodeResources{}
	if err := json.Unmarshal(n.Allocatable.Bytes, allocatable); err != nil {
		return nil, err
	}

	capacity := &model.KubernetesNodeResources{}
	if err := json.Unmarshal(n.Capacity.Bytes, capacity); err != nil {
		return nil, err
	}

	return &model.KubernetesNode{
		Name:                    n.Name,
		KernelVersion:           n.KernelVersion,
		OSImage:                 n.OsImage,
		ContainerRuntimeVersion: n.ContainerRuntimeVersion,
		KubeletVersion:          n.KubeletVersion,
		KubeProxyVersion:        n.KubeProxyVersion,
		OperatingSystem:         n.OperatingSystem,
		Architecture:            n.Architecture,
		Conditions:              conditions,
		Allocatable:             allocatable,
		Capacity:                capacity,
		InternalIP:              n.InternalIp,
	}, nil
}

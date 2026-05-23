package main

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type pgContainer struct {
	container *postgres.PostgresContainer
	teardown  func()
	connStr   string
}

func startPostgresContainer(ctx context.Context) (*pgContainer, error) {
	container, err := postgres.Run(ctx,
		imageName,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("running testcontainer: %w", err)
	}

	return setupPgContainerStruct(ctx, container)
}

func setupPgContainerStruct(
	ctx context.Context,
	container *postgres.PostgresContainer,
) (*pgContainer, error) {
	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		errTerm := testcontainers.TerminateContainer(container)
		if errTerm != nil {
			_ = errTerm
		}

		return nil, fmt.Errorf("getting connection string: %w", err)
	}

	return &pgContainer{
		connStr:   connStr,
		container: container,
		teardown: func() {
			errTerm := testcontainers.TerminateContainer(container)
			if errTerm != nil {
				_ = errTerm
			}
		},
	}, nil
}

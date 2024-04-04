package testdb

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	go_ora "github.com/sijms/go-ora/v2"
)

const (
	ORACLE_IMAGE   = "container-registry.oracle.com/database/free"
	ORACLE_VERSION = "latest"

	ORACLE_PASSWORD = "password"
)

func newOracle(opts ...OptionsFunc) (*sql.DB, func(), error) {
	option := &options{}
	for _, f := range opts {
		f(option)
	}
	// Uses a sensible default on windows (tcp/http) and linux/osx (socket).
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to docker: %v", err)
	}
	options := &dockertest.RunOptions{
		Repository: ORACLE_IMAGE,
		Tag:        ORACLE_VERSION,
		Env: []string{
			"ORACLE_PWD=" + ORACLE_PASSWORD,
		},
		Labels:       map[string]string{"goose_test": "1"},
		PortBindings: make(map[docker.Port][]docker.PortBinding),
	}
	if option.bindPort > 0 {
		options.PortBindings[docker.Port("1521/tcp")] = []docker.PortBinding{
			{HostPort: strconv.Itoa(option.bindPort)},
		}
	}
	container, err := pool.RunWithOptions(
		options,
		func(config *docker.HostConfig) {
			// Set AutoRemove to true so that stopped container goes away by itself.
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create docker container: %v", err)
	}
	cleanup := func() {
		if option.debug {
			// User must manually delete the Docker container.
			return
		}
		if err := pool.Purge(container); err != nil {
			log.Printf("failed to purge resource: %v", err)
		}
	}
	containerPort, err := strconv.Atoi(container.GetPort("1521/tcp"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse container port: %w", err)
	}
	uri := go_ora.BuildUrl("localhost", containerPort, "FREE", "system", ORACLE_PASSWORD, nil)
	var db *sql.DB
	// Exponential backoff-retry, because the application in the container
	// might not be ready to accept connections yet.
	if err := pool.Retry(
		func() error {
			var err error
			db, err = sql.Open("oracle", uri)
			if err != nil {
				return err
			}
			return db.Ping()
		},
	); err != nil {
		return nil, cleanup, fmt.Errorf("could not connect to docker database: %v", err)
	}
	return db, cleanup, nil
}

package clerk

import (
	docker "github.com/docker/docker/client"
)

// DockerAPIClient is an interface that clients that talk with a docker server must implement.
type DockerAPIClient interface {
	docker.APIClient
}

type Registry interface {
	ID() string
	Ping() error
	Register(service *Service) error
	Unregister(service *Service) error
	Refresh(service *Service) error
}

type Service struct {
	ID         string
	Name       string
	Port       int
	IP         string
	Tags       []string
	Attributes map[string]string
}

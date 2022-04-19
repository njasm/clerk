package clerk

import (
	docker "github.com/docker/docker/client"
	"github.com/njasm/clerk/internal/service"
)

// DockerAPIClient is an interface that clients that talk with a docker server must implement.
type DockerAPIClient interface {
	docker.APIClient
}

type Registry interface {
	ID() string
	Ping() error
	Register(service *service.Service) error
	Unregister(service *service.Service) error
	Services() ([]*service.RegisteredService, error)
}

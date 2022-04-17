package clerk

import (
	"strings"

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
	Services() ([]*Service, error)
}

type Service struct {
	ID     string
	Name   string
	IP     string
	Port   int
	Proto  string
	Config map[string]string
}

func (s *Service) Register() bool {
	data, exist := s.Config["com.github.njasm.clerk.register"]
	if !exist {
		return false
	}

	if strings.Trim(strings.ToLower(data), " ") == "true" {
		return true
	}

	return false
}

func (s *Service) GetConfig(keySuffix string) string {
	key := "com.github.njasm.clerk." + keySuffix
	return s.Config[key]
}

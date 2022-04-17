package registry

import (
	"errors"
	"fmt"
	"log"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	clerk "github.com/njasm/clerk/internal"
)

const consulID = "consul"

var ErrServiceIsNil = errors.New("Service is nil")

func NewConsul() (clerk.Registry, error) {
	config := consulapi.DefaultConfig()
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating consul registry: %w", err)
	}

	return &Consul{client: client}, nil
}

type Consul struct {
	client *consulapi.Client
}

func (c *Consul) ID() string {
	return consulID
}

func (c *Consul) Ping() error {
	status := c.client.Status()
	leader, err := status.Leader()
	if err != nil {
		return err
	}

	log.Println("consul: current leader ", leader)

	return nil
}

func (c *Consul) Register(service *clerk.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	tags := []string{}
	for key, value := range service.Config {
		if strings.ToLower(key) == "com.github.njasm.clerk.tags" {
			tags = append(tags, strings.Split(value, ",")...)
			continue
		}
	}

	config := convertMetadataKeys(service.Config)
	registration := consulapi.AgentServiceRegistration{
		Kind:    consulapi.ServiceKindTypical,
		Name:    service.Name,
		ID:      service.ID,
		Address: service.IP,
		Port:    service.Port,
		Tags:    tags,
		Meta:    config,
	}

	return c.client.Agent().ServiceRegister(&registration)
}

func (c *Consul) Unregister(service *clerk.Service) error {
	if service == nil {
		return ErrServiceIsNil
	}

	return c.client.Agent().ServiceDeregister(service.ID)
}

func (c *Consul) Refresh(service *clerk.Service) error {
	return nil
}

func (c *Consul) Services() ([]*clerk.Service, error) {
	rv := []*clerk.Service{}
	services, err := c.client.Agent().Services()
	if err != nil {
		return rv, err
	}

	for _, value := range services {
		config := revertMetadataKeys(value.Meta)
		s := &clerk.Service{
			ID:     value.ID,
			Name:   value.Service,
			Port:   value.Port,
			IP:     value.Address,
			Config: config,
		}

		rv = append(rv, s)
	}

	return rv, nil
}

func convertMetadataKeys(m map[string]string) map[string]string {
	return metadataReplace(m, ".", "_")
}

func revertMetadataKeys(m map[string]string) map[string]string {
	return metadataReplace(m, "_", ".")
}

func metadataReplace(m map[string]string, old, new string) map[string]string {
	rv := map[string]string{}
	for k, v := range m {
		rv[strings.ReplaceAll(k, old, new)] = v
	}

	return rv
}

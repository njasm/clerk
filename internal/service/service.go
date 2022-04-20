package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	"github.com/njasm/clerk/internal/constants"
)

type RegisteredService struct {
	ID         string
	Name       string
	IP         string
	Port       int
	Proto      string
	Tags       []string
	Attributes map[string]string
	Config     map[string]string
}

type Service struct {
	id         string
	name       string
	tags       []string
	attributes map[string]string
	config     map[string]string
	container  types.ContainerJSON
	instances  map[string]Instance
}

type Instance struct {
	ID    string
	IP    string
	Port  int
	Proto string
}

func NewFrom(container types.ContainerJSON) *Service {
	srv := &Service{
		tags:       []string{},
		attributes: map[string]string{},
		config:     map[string]string{},
		container:  container,
		instances:  map[string]Instance{},
	}

	return srv.setConfig().
		setServiceName().
		setTags().
		setAttributes().
		setInstances()
}

func (s *Service) ID() string {
	return s.id
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) IPAddress() string {
	if instance, ok := s.instances[s.id]; ok {
		return instance.IP
	}

	return ""
}

func (s *Service) Port() int {
	if instance, ok := s.instances[s.id]; ok {
		return instance.Port
	}

	return 0
}

func (s *Service) IP() string {
	if instance, ok := s.instances[s.id]; ok {
		return instance.IP
	}

	return ""
}

func (s *Service) Config() map[string]string {
	return s.config
}

func (s *Service) GetConfig(keySuffix string) (string, bool) {
	var key string
	if strings.HasPrefix(keySuffix, constants.CONFIG_PREFIX) {
		key = keySuffix
	} else {
		key = constants.CONFIG_PREFIX + keySuffix
	}

	data, ok := s.config[key]

	return data, ok
}

func (s *Service) Tags() []string {
	return s.tags
}

func (s *Service) Attributes() map[string]string {
	return s.attributes
}

func (s *Service) GetAttribute(key string) (string, bool) {
	data, ok := s.attributes[key]

	return data, ok
}

func (s *Service) Instances() map[string]Instance {
	return s.instances
}

func (s *Service) Register() bool {
	data, exist := s.GetConfig(constants.CONFIG_CLERK_REGISTER)
	if !exist {
		return false
	}

	// if we have no instances, then there's
	// no network address nor port defined
	if len(s.instances) == 0 {
		return false
	}

	if trimAndLowerString(data) == "true" {
		return true
	}

	return false
}

func (s *Service) setConfig() *Service {
	config := map[string]string{}
	if s.container.Config == nil {
		return s
	}

	for key, value := range s.container.Config.Labels {
		key = trimAndLowerString(key)
		if strings.HasPrefix(key, constants.CONFIG_PREFIX) {
			config[key] = value
		}
	}

	s.config = config
	return s
}

func (s *Service) setServiceName() *Service {
	name, ok := s.GetConfig(constants.CONFIG_SERVICE_NAME)
	if ok {
		s.name = strings.TrimLeft(name, "/")
		return s
	}

	s.name = strings.TrimLeft(s.container.Name, "/")
	return s
}

func (s *Service) setTags() *Service {
	tags, ok := s.GetConfig(constants.CONFIG_SERVICE_TAGS)
	if ok {
		s.tags = append(s.tags, strings.Split(tags, ",")...)
	}

	return s
}

func (s *Service) setAttributes() *Service {
	attrs, ok := s.GetConfig(constants.CONFIG_SERVICE_ATTRIBUTES)
	if ok {
		for _, value := range strings.Split(attrs, ",") {
			data := strings.Split(value, ":")
			key := ""
			for _, val := range data {
				if key == "" {
					key = val
					continue
				}

				s.attributes[key] = val
				key = ""
			}
		}
	}

	return s
}

func (s *Service) setInstances() *Service {
	// if we have explicit ports defined, use those
	ports, ok := s.GetConfig(constants.CONFIG_SERVICE_PORTS)
	if ports != "" && ok {
		for _, portProtoPair := range strings.Split(ports, ",") {
			err := instance(s, portProtoPair)
			if err != nil {
				fmt.Println(fmt.Errorf("error: %w", err))
				continue
			}
		}

		return s
	}

	for portProtoPair := range s.container.Config.ExposedPorts {
		rawPort := string(portProtoPair)
		err := instance(s, rawPort)
		if err != nil {
			fmt.Println(fmt.Errorf("error: %w", err))
			continue
		}
	}

	return s
}

var ErrAtoi = errors.New("converting to int")
var ErrNoNetworkEndpointSettings = errors.New("no network endpoint settings")

func instance(s *Service, rawPort string) error {
	proto, port := nat.SplitProtoPort(rawPort)
	intPort, err := strconv.Atoi(port)
	if err != nil {
		return ErrAtoi
	}

	serviceID := fmt.Sprintf("%v:%v:%v:%v", s.name, proto, port, s.container.Config.Hostname)
	for _, networkValue := range s.container.NetworkSettings.Networks {
		if networkValue == nil {
			return ErrNoNetworkEndpointSettings
		}

		// set Service ID to the first instance ID
		if s.id == "" {
			s.id = serviceID
		}

		s.instances[serviceID] = Instance{
			ID:    serviceID,
			IP:    networkValue.IPAddress,
			Port:  intPort,
			Proto: proto,
		}
	}

	return nil
}

func trimAndLowerString(data string) string {
	return strings.Trim(strings.ToLower(data), " ")
}

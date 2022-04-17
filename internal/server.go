package clerk

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Server struct {
	dockerClient          DockerAPIClient
	dockerMessagesChannel <-chan events.Message
	dockerErrorChannel    <-chan error
	stopServerChannel     <-chan bool
	registry              Registry
}

func New(stopServer <-chan bool, registry Registry) (*Server, error) {
	client, err := dockerapi.NewClientWithOpts(dockerapi.FromEnv)
	if err != nil {
		return nil, err
	}

	chMessages, chErrors := client.Events(context.TODO(), types.EventsOptions{
		Filters: filters.NewArgs(
			filters.KeyValuePair{
				Key: "Type", Value: "container",
			},
		),
	})

	return &Server{
		dockerClient:          client,
		dockerMessagesChannel: chMessages,
		dockerErrorChannel:    chErrors,
		stopServerChannel:     stopServer,
		registry:              registry,
	}, nil
}

func (s *Server) Start() {

	timer := time.NewTicker(time.Second * 2)
	for {
		select {
		case data := <-s.dockerMessagesChannel:

			fmt.Printf("Event ID: %+v\n", data.ID)
			fmt.Printf("Status  : %+v\n", data.Status)
			fmt.Printf("Scope   : %+v\n", data.Scope)
			fmt.Printf("Type    : %+v\n", data.Type)
			fmt.Printf("From    : %+v\n", data.From)
			fmt.Printf("Action  : %+v\n", data.Action)
			fmt.Printf("Actor ID: %+v\n", data.Actor.ID)
			fmt.Println("A. Attrb:")

			for k, v := range data.Actor.Attributes {
				fmt.Printf(" - %+v : %+v\n", k, v)
			}

			println()

			if data.Status == "start" && data.Type == "container" {
				err := s.register(data.Actor.ID)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					fmt.Println(err)
				}
			}

			if data.Status == "die" && data.Type == "container" {
				err := s.unregister(data.Actor.ID)
				if err != nil {
					err = fmt.Errorf("error: %w", err)
					log.Println(err)
				}
			}

		case e := <-s.dockerErrorChannel:

			err := fmt.Errorf("error: %w", e)
			fmt.Println(err)

		case <-timer.C:

			fmt.Println("TIMER TICK - SYNCHRONIZATION")

			containers, err := s.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
			if err != nil {
				err = fmt.Errorf("error: %w", err)
				fmt.Println(err)
				continue
			}

			for _, v := range containers {
				fmt.Printf("%+v\n", v)
			}

			err = s.synchronise(containers)
			if err != nil {
				err = fmt.Errorf("error: %w", err)
				fmt.Println(err)
			}

		case <-s.stopServerChannel:

			//TODO: teardown gracefully everywere
			fmt.Println("Tearing Down service")
			timer.Stop()
			return

		default:

			fmt.Println("No events to process")
			time.Sleep(time.Duration(time.Second * 1))
			continue

		}
	}
}

func (s *Server) register(containerID string) error {
	services, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	for _, service := range services {
		if !service.Register() {
			return nil
		}

		err = s.registry.Register(service)
		if err != nil {
			log.Println(fmt.Errorf("error registering service: %w", err))
			err = nil
		}

	}

	return nil
}

func (s *Server) unregister(containerID string) error {
	services, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	for _, service := range services {
		err = s.registry.Unregister(service)
		if err != nil {
			log.Println(fmt.Errorf("error unregistering service: %w", err))
			err = nil
		}
	}

	return nil
}

func (s *Server) containerToService(containerID string) ([]*Service, error) {
	containerJson, err := s.dockerClient.ContainerInspect(context.TODO(), containerID)
	if err != nil {
		return nil, err
	}

	config := map[string]string{}
	for key, value := range containerJson.Config.Labels {
		key := strings.ToLower(key)
		if strings.HasPrefix(key, "com.github.njasm.clerk.") {
			config[key] = value
		}
	}

	rv := []*Service{}
	serviceName := strings.TrimLeft(containerJson.Name, "/")
	for key := range containerJson.Config.ExposedPorts {
		proto, port := nat.SplitProtoPort(string(key))
		intPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}

		serviceID := fmt.Sprintf("%v:%v:%v:%v", serviceName, proto, port, containerJson.Config.Hostname)

		for _, networkValue := range containerJson.NetworkSettings.Networks {
			if networkValue == nil {
				continue
			}

			if networkValue.IPAddress == "" {
				continue
			}

			service := Service{
				ID:     serviceID,
				Name:   serviceName,
				IP:     networkValue.IPAddress,
				Port:   intPort,
				Proto:  proto,
				Config: config,
			}

			rv = append(rv, &service)
		}

	}

	return rv, nil
}

func (s *Server) synchronise(containers []types.Container) error {
	if len(containers) == 0 {
		return nil
	}

	services, err := s.registry.Services()
	fmt.Println(services)
	if err != nil {
		err = fmt.Errorf("error getting services from registry: %w", err)
		fmt.Println(err)
		return err
	}

	for _, container := range containers {
		err := s.register(container.ID)
		if err != nil {
			err = fmt.Errorf("error: %w", err)
			fmt.Println(err)
			continue
		}
	}

	return nil
}

func ExitOnError(e error) {
	if e != nil {
		err := fmt.Errorf("error: %w", e)
		fmt.Println(err)
		os.Exit(1)
	}
}

package clerk

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerapi "github.com/docker/docker/client"
	"github.com/njasm/clerk/internal/service"
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
	service, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	if !service.Register() {
		return nil
	}

	err = s.registry.Register(service)
	if err != nil {
		log.Println(fmt.Errorf("error registering service: %w", err))
		err = nil
	}

	return nil
}

func (s *Server) unregister(containerID string) error {
	service, err := s.containerToService(containerID)
	if err != nil {
		return err
	}

	err = s.registry.Unregister(service)
	if err != nil {
		log.Println(fmt.Errorf("error unregistering service: %w", err))
		err = nil
	}

	return nil
}

func (s *Server) containerToService(containerID string) (*service.Service, error) {
	containerJson, err := s.dockerClient.ContainerInspect(context.TODO(), containerID)
	if err != nil {
		return nil, err
	}

	return service.NewFrom(containerJson), nil
}

func (s *Server) synchronise(containers []types.Container) error {
	if len(containers) == 0 {
		return nil
	}

	_, err := s.registry.Services()
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

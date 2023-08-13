package test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serviceA struct {
	serviceB ServiceB
}

func (s serviceA) Foo() {}

type serviceB struct{}

func (s serviceB) Bar() {}

func TestServiceLocator(t *testing.T) {
	registry := NewServiceRegistry()

	registry.RegisterServiceA("service", func(_ string, serviceLocator ServiceLocator) (ServiceA, error) {
		serviceB, err := serviceLocator.GetServiceB("service")
		if err != nil {
			return nil, err
		}

		return serviceA{
			serviceB: serviceB,
		}, nil
	})

	registry.RegisterServiceB("service", func(_ string, serviceLocator ServiceLocator) (ServiceB, error) {
		return serviceB{}, nil
	})

	service, err := registry.GetServiceA("service")

	require.NoError(t, err)

	assert.Equal(t, service, serviceA{
		serviceB: serviceB{},
	})
}

func TestServiceFactoryNotFound(t *testing.T) {
	registry := NewServiceRegistry()

	_, err := registry.GetServiceA("service")

	assert.ErrorContains(t, err, "no factory registered for ServiceA with name 'service'")
}

func TestServiceFactoryFailed(t *testing.T) {
	registry := NewServiceRegistry()

	registry.RegisterServiceA("service", func(_ string, serviceLocator ServiceLocator) (ServiceA, error) {
		return nil, errors.New("failed to create service")
	})

	_, err := registry.GetServiceA("service")

	assert.ErrorContains(t, err, "failed to create service")
}

func TestCircularDependencyDetection(t *testing.T) {
	registry := NewServiceRegistry()

	registry.RegisterServiceA("service", func(_ string, serviceLocator ServiceLocator) (ServiceA, error) {
		_, err := serviceLocator.GetServiceB("service")
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	registry.RegisterServiceB("service", func(_ string, serviceLocator ServiceLocator) (ServiceB, error) {
		_, err := serviceLocator.GetServiceA("service")
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	_, err := registry.GetServiceA("service")

	assert.Equal(t, CircularDependencyError{
		ServiceType:     "ServiceA",
		ServiceName:     "service",
		DependencyGraph: []string{"ServiceA:service", "ServiceB:service"},
	}, err)
}

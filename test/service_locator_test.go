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

	registry.RegisterServiceA(func(serviceLocator ServiceLocator) (ServiceA, error) {
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

	service, err := registry.GetServiceA()

	require.NoError(t, err)

	assert.Equal(t, serviceA{
		serviceB: serviceB{},
	}, service)
}

func TestServiceFactoryNotFound(t *testing.T) {
	registry := NewServiceRegistry()

	_, err := registry.GetServiceA()

	assert.ErrorContains(t, err, "no factory registered for ServiceA")
}

func TestServiceFactoryFailed(t *testing.T) {
	registry := NewServiceRegistry()

	registry.RegisterServiceA(func(serviceLocator ServiceLocator) (ServiceA, error) {
		return nil, errors.New("failed to create service")
	})

	_, err := registry.GetServiceA()

	assert.ErrorContains(t, err, "failed to create service")
}

func TestCircularDependencyDetection(t *testing.T) {
	registry := NewServiceRegistry()

	registry.RegisterServiceA(func(serviceLocator ServiceLocator) (ServiceA, error) {
		_, err := serviceLocator.GetServiceB("service")
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	registry.RegisterServiceB("service", func(_ string, serviceLocator ServiceLocator) (ServiceB, error) {
		_, err := serviceLocator.GetServiceA()
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	_, err := registry.GetServiceA()

	assert.Equal(t, CircularDependencyError{
		ServiceType:     "ServiceA",
		ServiceName:     "",
		DependencyGraph: []string{"ServiceA", "ServiceB:service"},
	}, err)
}

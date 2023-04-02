package test

import (
	"fmt"
	"strings"
	"sync"
)

// ServiceLocator locates named services in a type-safe manner.
type ServiceLocator interface {
	GetServiceA(name string) (ServiceA, error)
	GetServiceB(name string) (ServiceB, error)
}

type ServiceAFactory func(ServiceLocator) (ServiceA, error)
type ServiceBFactory func(ServiceLocator) (ServiceB, error)

// ServiceRegistry allows registering service factories to construct new instances of a service.
// ServiceRegistry is also the primary {ServiceLocator} entrypoint.
type ServiceRegistry struct {
	mu sync.Mutex

	instancesServiceA map[string]ServiceA
	factoriesServiceA map[string]ServiceAFactory
	instancesServiceB map[string]ServiceB
	factoriesServiceB map[string]ServiceBFactory
}

// NewServiceRegistry instantiates a new {ServiceRegistry}.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{instancesServiceA: make(map[string]ServiceA), factoriesServiceA: make(map[string]ServiceAFactory), instancesServiceB: make(map[string]ServiceB), factoriesServiceB: make(map[string]ServiceBFactory)}
}

func (r *ServiceRegistry) RegisterServiceA(serviceName string, factory ServiceAFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factoriesServiceA[serviceName] = factory
}

func (r *ServiceRegistry) GetServiceA(serviceName string) (ServiceA, error) {
	return r.getServiceA(serviceName, newServiceLocationContext(r, 0))
}

func (r *ServiceRegistry) getServiceA(serviceName string, ctx *serviceLocationContext) (ServiceA, error) {
	r.mu.Lock()
	instance, instanceOk := r.instancesServiceA[serviceName]
	factory, factoryOk := r.factoriesServiceA[serviceName]
	r.mu.Unlock()

	if instanceOk {
		return instance, nil
	}

	if ctx.isVisitedServiceA(serviceName) {
		return nil, newCircularDependencyError("ServiceA", serviceName, ctx.dependencyGraph)
	}
	ctx.markVisitedServiceA(serviceName)

	if !factoryOk {
		return nil, fmt.Errorf("no factory registered for ServiceA with name '%s'", serviceName)
	}

	instance, err := factory(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.instancesServiceA[serviceName] = instance
	r.mu.Unlock()

	return instance, nil
}

func (r *ServiceRegistry) RegisterServiceB(serviceName string, factory ServiceBFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factoriesServiceB[serviceName] = factory
}

func (r *ServiceRegistry) GetServiceB(serviceName string) (ServiceB, error) {
	return r.getServiceB(serviceName, newServiceLocationContext(r, 0))
}

func (r *ServiceRegistry) getServiceB(serviceName string, ctx *serviceLocationContext) (ServiceB, error) {
	r.mu.Lock()
	instance, instanceOk := r.instancesServiceB[serviceName]
	factory, factoryOk := r.factoriesServiceB[serviceName]
	r.mu.Unlock()

	if instanceOk {
		return instance, nil
	}

	if ctx.isVisitedServiceB(serviceName) {
		return nil, newCircularDependencyError("ServiceB", serviceName, ctx.dependencyGraph)
	}
	ctx.markVisitedServiceB(serviceName)

	if !factoryOk {
		return nil, fmt.Errorf("no factory registered for ServiceB with name '%s'", serviceName)
	}

	instance, err := factory(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.instancesServiceB[serviceName] = instance
	r.mu.Unlock()

	return instance, nil
}

type serviceLocationContext struct {
	registry        *ServiceRegistry
	dependencyGraph []string

	visitLock sync.Mutex

	visitedServiceA map[string]bool
	visitedServiceB map[string]bool
}

func newServiceLocationContext(registry *ServiceRegistry, maxDepth int) *serviceLocationContext {
	return &serviceLocationContext{registry: registry, visitedServiceA: make(map[string]bool), visitedServiceB: make(map[string]bool)}
}

func (c *serviceLocationContext) GetServiceA(serviceName string) (ServiceA, error) {
	return c.registry.getServiceA(serviceName, c)
}

func (c *serviceLocationContext) isVisitedServiceA(serviceName string) bool {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	return c.visitedServiceA[serviceName]
}

func (c *serviceLocationContext) markVisitedServiceA(serviceName string) {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	c.visitedServiceA[serviceName] = true
	c.dependencyGraph = append(c.dependencyGraph, "ServiceA:"+serviceName)
}

func (c *serviceLocationContext) GetServiceB(serviceName string) (ServiceB, error) {
	return c.registry.getServiceB(serviceName, c)
}

func (c *serviceLocationContext) isVisitedServiceB(serviceName string) bool {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	return c.visitedServiceB[serviceName]
}

func (c *serviceLocationContext) markVisitedServiceB(serviceName string) {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	c.visitedServiceB[serviceName] = true
	c.dependencyGraph = append(c.dependencyGraph, "ServiceB:"+serviceName)
}

type CircularDependencyError struct {
	ServiceType     string
	ServiceName     string
	DependencyGraph []string
}

func newCircularDependencyError(serviceType string, serviceName string, dependencyGraph []string) CircularDependencyError {
	return CircularDependencyError{
		DependencyGraph: dependencyGraph,
		ServiceName:     serviceName,
		ServiceType:     serviceType,
	}
}
func (e CircularDependencyError) Error() string {
	dependencyPath := strings.Join(e.DependencyGraph, " -> ")
	return fmt.Sprintf("circular dependency detected for %s '%s': %s", e.ServiceType, e.ServiceName, dependencyPath)
}

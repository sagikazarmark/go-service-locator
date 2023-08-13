package test

import (
	"errors"
	"fmt"
	subtest "github.com/sagikazarmark/go-service-locator/test/subtest"
	"strings"
	"sync"
)

// ServiceFactory creates a new instance of T.
type ServiceFactory[T any] func(ServiceLocator) (T, error)

// NamedServiceFactory creates a new named instance of T.
type NamedServiceFactory[T any] func(string, ServiceLocator) (T, error)

// ServiceRegistry allows registering service factories to construct new instances of a service.
// ServiceRegistry is also the primary {ServiceLocator} entrypoint.
type ServiceRegistry struct {
	mu sync.Mutex

	instanceServiceA  ServiceA
	factoryServiceA   ServiceFactory[ServiceA]
	instancesServiceB map[string]ServiceB
	factoriesServiceB map[string]NamedServiceFactory[ServiceB]
	instanceServiceC  subtest.ServiceC
	factoryServiceC   ServiceFactory[subtest.ServiceC]
}

// NewServiceRegistry instantiates a new {ServiceRegistry}.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{instancesServiceB: make(map[string]ServiceB), factoriesServiceB: make(map[string]NamedServiceFactory[ServiceB])}
}

// RegisterServiceA registers a factory for {ServiceA}.
func (r *ServiceRegistry) RegisterServiceA(factory ServiceFactory[ServiceA]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factoryServiceA = factory
}

// GetServiceA retrieves an instance of {ServiceA}.
func (r *ServiceRegistry) GetServiceA() (ServiceA, error) {
	return r.getServiceA(newServiceLocationContext(r, 0))
}

func (r *ServiceRegistry) getServiceA(ctx *serviceLocationContext) (ServiceA, error) {
	r.mu.Lock()
	instance := r.instanceServiceA
	instanceOk := instance != nil
	factory := r.factoryServiceA
	factoryOk := factory != nil
	r.mu.Unlock()

	if instanceOk {
		return instance, nil
	}

	if ctx.isVisitedServiceA() {
		return nil, newCircularDependencyError("ServiceA", "", ctx.dependencyGraph)
	}
	ctx.markVisitedServiceA()

	if !factoryOk {
		return nil, errors.New("no factory registered for ServiceA")
	}

	instance, err := factory(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.instanceServiceA = instance
	r.mu.Unlock()

	return instance, nil
}

// RegisterServiceB registers a factory for {ServiceB}.
func (r *ServiceRegistry) RegisterServiceB(serviceName string, factory NamedServiceFactory[ServiceB]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factoriesServiceB[serviceName] = factory
}

// GetServiceB retrieves an instance of {ServiceB}.
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

	instance, err := factory(serviceName, ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.instancesServiceB[serviceName] = instance
	r.mu.Unlock()

	return instance, nil
}

// RegisterServiceC registers a factory for {ServiceC}.
func (r *ServiceRegistry) RegisterServiceC(factory ServiceFactory[subtest.ServiceC]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factoryServiceC = factory
}

// GetServiceC retrieves an instance of {ServiceC}.
func (r *ServiceRegistry) GetServiceC() (subtest.ServiceC, error) {
	return r.getServiceC(newServiceLocationContext(r, 0))
}

func (r *ServiceRegistry) getServiceC(ctx *serviceLocationContext) (subtest.ServiceC, error) {
	r.mu.Lock()
	instance := r.instanceServiceC
	instanceOk := instance != nil
	factory := r.factoryServiceC
	factoryOk := factory != nil
	r.mu.Unlock()

	if instanceOk {
		return instance, nil
	}

	if ctx.isVisitedServiceC() {
		return nil, newCircularDependencyError("ServiceC", "", ctx.dependencyGraph)
	}
	ctx.markVisitedServiceC()

	if !factoryOk {
		return nil, errors.New("no factory registered for ServiceC")
	}

	instance, err := factory(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.instanceServiceC = instance
	r.mu.Unlock()

	return instance, nil
}

type serviceLocationContext struct {
	registry        *ServiceRegistry
	dependencyGraph []string

	visitLock sync.Mutex

	visitedServiceA bool
	visitedServiceB map[string]bool
	visitedServiceC bool
}

func newServiceLocationContext(registry *ServiceRegistry, maxDepth int) *serviceLocationContext {
	return &serviceLocationContext{registry: registry, visitedServiceB: make(map[string]bool)}
}

func (c *serviceLocationContext) GetServiceA() (ServiceA, error) {
	return c.registry.getServiceA(c)
}

func (c *serviceLocationContext) isVisitedServiceA() bool {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	return c.visitedServiceA
}

func (c *serviceLocationContext) markVisitedServiceA() {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	c.visitedServiceA = true
	c.dependencyGraph = append(c.dependencyGraph, "ServiceA")
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

func (c *serviceLocationContext) GetServiceC() (subtest.ServiceC, error) {
	return c.registry.getServiceC(c)
}

func (c *serviceLocationContext) isVisitedServiceC() bool {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	return c.visitedServiceC
}

func (c *serviceLocationContext) markVisitedServiceC() {
	c.visitLock.Lock()
	defer c.visitLock.Unlock()

	c.visitedServiceC = true
	c.dependencyGraph = append(c.dependencyGraph, "ServiceC")
}

// CircularDependencyError is returned when there is a circular dependency between two services.
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

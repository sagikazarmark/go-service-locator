package test

import "github.com/sagikazarmark/go-service-locator/test/subtest"

// ServiceLocator locates named services in a type-safe manner.
type ServiceLocator interface {
	GetServiceA() (ServiceA, error)
	GetServiceB(name string) (ServiceB, error)
	GetServiceC() (subtest.ServiceC, error)
}

// ServiceA is an example for service locator tests.
type ServiceA interface {
	Foo()
}

// ServiceB is an example for service locator tests.
type ServiceB interface {
	Bar()
}

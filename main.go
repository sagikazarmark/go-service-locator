package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"
	"golang.org/x/exp/slices"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "insufficient arguments")
		fmt.Fprintln(os.Stderr, "usage: <command> PACKAGE SERVICE_TYPE [SERVICE_TYPE...]")
	}

	pkg := os.Args[1]
	services := os.Args[2:]

	serviceDefinitions := make([]serviceDefinition, 0, len(services))

	for _, service := range services {
		serviceSegments := strings.SplitN(service, ":", 2)

		def := serviceDefinition{
			name: serviceSegments[0],
		}

		if len(serviceSegments) > 1 {
			serviceParams := strings.Split(serviceSegments[1], ",")

			if slices.Contains(serviceParams, "named") {
				def.named = true
			}
		}

		serviceDefinitions = append(serviceDefinitions, def)
	}

	f := jen.NewFile(pkg)
	f.ImportName("sync", "sync")
	f.ImportName("fmt", "fmt")
	f.ImportName("strings", "strings")

	generateServiceLocator(f, serviceDefinitions)
	generateGenericServiceFactory(f)
	generateGenericNamedServiceFactory(f)
	generateServiceRegistry(f, serviceDefinitions)
	generateServiceLocationContext(f, serviceDefinitions)
	generateCircularDependencyError(f)

	err := f.Render(os.Stdout)
	if err != nil {
		fmt.Fprint(os.Stderr, "Error generating the code:", err)
		os.Exit(1)
	}
}

type serviceDefinition struct {
	name  string
	named bool
}

// helper functions
func ifNamed(named bool, g *jen.Group, args ...jen.Code) {
	if named {
		g.Add(args...)
	}
}

func ifNamedFunc(named bool, args ...jen.Code) func(g *jen.Group) {
	return func(g *jen.Group) {
		ifNamed(named, g, args...)
	}
}

func generateServiceLocator(f *jen.File, services []serviceDefinition) {
	f.Comment("ServiceLocator locates named services in a type-safe manner.")
	f.Type().Id("ServiceLocator").InterfaceFunc(func(g *jen.Group) {
		for _, service := range services {
			g.Id("Get"+service.name).ParamsFunc(ifNamedFunc(service.named, jen.Id("name").String())).Params(jen.Id(service.name), jen.Error())
			// if service.named {
			// 	g.Id("Get"+service.name).Params(jen.Id("name").String()).Params(jen.Id(service.name), jen.Error())
			// } else {
			// 	g.Id("Get"+service.name).Params().Params(jen.Id(service.name), jen.Error())
			// }
		}
	})
}

func generateGenericServiceFactory(f *jen.File) {
	f.Comment("ServiceFactory creates a new instance of T.")
	f.Type().Id("ServiceFactory").Types(jen.Id("T").Any()).Func().Params(jen.Id("ServiceLocator")).Params(jen.Id("T"), jen.Error())
}

func generateGenericNamedServiceFactory(f *jen.File) {
	f.Comment("NamedServiceFactory creates a new named instance of T.")
	f.Type().Id("NamedServiceFactory").Types(jen.Id("T").Any()).Func().Params(jen.String(), jen.Id("ServiceLocator")).Params(jen.Id("T"), jen.Error())
}

func generateServiceRegistry(f *jen.File, services []serviceDefinition) {
	f.Comment("ServiceRegistry allows registering service factories to construct new instances of a service.")
	f.Comment("ServiceRegistry is also the primary {ServiceLocator} entrypoint.")
	f.Type().Id("ServiceRegistry").StructFunc(func(g *jen.Group) {
		g.Id("mu").Qual("sync", "Mutex")
		g.Line()

		for _, service := range services {
			if service.named {
				g.Id("instances" + service.name).Map(jen.String()).Id(service.name)
				g.Id("factories" + service.name).Map(jen.String()).Id("NamedServiceFactory").Types(jen.Id(service.name))
			} else {
				g.Id("instance" + service.name).Id(service.name)
				g.Id("factory" + service.name).Id("ServiceFactory").Types(jen.Id(service.name))
			}
		}
	})

	f.Comment("NewServiceRegistry instantiates a new {ServiceRegistry}.")
	f.Func().Id("NewServiceRegistry").Params().Op("*").Id("ServiceRegistry").Block(
		jen.Return(jen.Op("&").Id("ServiceRegistry").ValuesFunc(func(g *jen.Group) {
			for _, service := range services {
				if service.named {
					g.Id("instances" + service.name).Op(":").Make(jen.Map(jen.String()).Id(service.name))
					g.Id("factories" + service.name).Op(":").Make(jen.Map(jen.String()).Id("NamedServiceFactory").Types(jen.Id(service.name)))
				}
			}
		})),
	)

	generateServiceRegistryMethods(f, services)
}

func generateServiceRegistryMethods(f *jen.File, services []serviceDefinition) {
	for _, service := range services {
		f.Line()

		// Register method
		f.Commentf("Register%s registers a factory for {%s}.", service.name, service.name)
		f.Func().
			Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("Register" + service.name).
			ParamsFunc(func(g *jen.Group) {
				if service.named {
					g.Id("serviceName").String()
					g.Id("factory").Id("NamedServiceFactory").Types(jen.Id(service.name))
				} else {
					g.Id("factory").Id("ServiceFactory").Types(jen.Id(service.name))
				}
			}).
			BlockFunc(func(g *jen.Group) {
				g.Id("r").Dot("mu").Dot("Lock").Call()
				g.Defer().Id("r").Dot("mu").Dot("Unlock").Call()
				g.Line()

				if service.named {
					g.Id("r").Dot("factories" + service.name).Index(jen.Id("serviceName")).Op("=").Id("factory")
				} else {
					g.Id("r").Dot("factory" + service.name).Op("=").Id("factory")
				}
			})

		f.Line()

		// Get method
		f.Commentf("Get%s retrieves an instance of {%s}.", service.name, service.name)
		f.Func().
			Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("Get"+service.name).
			ParamsFunc(ifNamedFunc(service.named, jen.Id("serviceName").String())).
			Params(jen.Id(service.name), jen.Error()).
			BlockFunc(func(g *jen.Group) {
				g.Return().Id("r").Dot("get" + service.name).CallFunc(func(g *jen.Group) {
					ifNamed(service.named, g, jen.Id("serviceName"))
					g.Id("newServiceLocationContext").Call(jen.Id("r"), jen.Lit(0))
				})
			})

		f.Line()

		// Private get method
		f.Func().
			Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("get"+service.name).
			ParamsFunc(func(g *jen.Group) {
				ifNamed(service.named, g, jen.Id("serviceName").String())
				g.Id("ctx").Op("*").Id("serviceLocationContext")
			}).
			Params(jen.Id(service.name), jen.Error()).
			BlockFunc(func(g *jen.Group) {
				g.Id("r").Dot("mu").Dot("Lock").Call()

				if service.named {
					g.Id("instance, instanceOk").Op(":=").Id("r").Dot("instances" + service.name).Index(jen.Id("serviceName"))
					g.Id("factory, factoryOk").Op(":=").Id("r").Dot("factories" + service.name).Index(jen.Id("serviceName"))
				} else {
					g.Id("instance").Op(":=").Id("r").Dot("instance" + service.name)
					g.Id("instanceOk").Op(":=").Id("instance").Op("!=").Nil()
					g.Id("factory").Op(":=").Id("r").Dot("factory" + service.name)
					g.Id("factoryOk").Op(":=").Id("factory").Op("!=").Nil()
				}

				g.Id("r").Dot("mu").Dot("Unlock").Call()

				g.Line()

				g.If(jen.Id("instanceOk")).Block(
					jen.Return(jen.Id("instance"), jen.Nil()),
				)

				g.Line()

				g.If(jen.Id("ctx").Dot("isVisited" + service.name).CallFunc(ifNamedFunc(service.named, jen.Id("serviceName")))).Block(
					jen.Return(
						jen.Nil(),
						jen.Id("newCircularDependencyError").CallFunc(func(g *jen.Group) {
							g.Lit(service.name)
							if service.named {
								g.Id("serviceName")
							} else {
								g.Lit("")
							}
							g.Id("ctx").Dot("dependencyGraph")
						}),
					),
				)

				g.Id("ctx").Dot("markVisited" + service.name).CallFunc(ifNamedFunc(service.named, jen.Id("serviceName")))

				g.Line()

				g.If(jen.Op("!").Id("factoryOk")).BlockFunc(func(g *jen.Group) {
					if service.named {
						g.Return(jen.Nil(), jen.Qual("fmt", "Errorf").Call(
							jen.Lit("no factory registered for "+service.name+" with name '%s'"),
							jen.Id("serviceName"),
						))
					} else {
						g.Return(jen.Nil(), jen.Qual("errors", "New").Call(
							jen.Lit("no factory registered for "+service.name),
						))
					}
				})

				g.Line()

				g.Id("instance, err").Op(":=").Id("factory").CallFunc(func(g *jen.Group) {
					ifNamed(service.named, g, jen.Id("serviceName"))
					g.Id("ctx")
				})
				g.If(jen.Id("err").Op("!=").Nil()).Block(
					jen.Return(jen.Nil(), jen.Id("err")),
				)

				g.Line()

				g.Id("r").Dot("mu").Dot("Lock").Call()
				if service.named {
					g.Id("r").Dot("instances" + service.name).Index(jen.Id("serviceName")).Op("=").Id("instance")
				} else {
					g.Id("r").Dot("instance" + service.name).Op("=").Id("instance")
				}
				g.Id("r").Dot("mu").Dot("Unlock").Call()

				g.Line()

				g.Return(jen.Id("instance"), jen.Nil())
			})
	}
}

func generateServiceLocationContext(f *jen.File, services []serviceDefinition) {
	f.Type().Id("serviceLocationContext").StructFunc(func(g *jen.Group) {
		g.Id("registry").Op("*").Id("ServiceRegistry")
		g.Id("dependencyGraph").Index().String()

		g.Line()

		g.Id("visitLock").Qual("sync", "Mutex")

		g.Line()

		for _, service := range services {
			if service.named {
				g.Id("visited" + service.name).Map(jen.String()).Bool()
			} else {
				g.Id("visited" + service.name).Bool()
			}
		}
	})

	f.Func().Id("newServiceLocationContext").Params(jen.Id("registry").Op("*").Id("ServiceRegistry"), jen.Id("maxDepth").Int()).Op("*").Id("serviceLocationContext").Block(
		jen.Return(jen.Op("&").Id("serviceLocationContext").ValuesFunc(func(g *jen.Group) {
			g.Id("registry").Op(":").Id("registry")

			for _, service := range services {
				if service.named {
					g.Id("visited" + service.name).Op(":").Make(jen.Map(jen.String()).Bool())
				}
			}
		})),
	)

	for _, service := range services {
		f.Line()

		// Get method
		f.Func().
			Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("Get"+service.name).
			ParamsFunc(ifNamedFunc(service.named, jen.Id("serviceName").String())).
			Params(jen.Id(service.name), jen.Error()).
			Block(
				jen.Return(jen.Id("c").Dot("registry").Dot("get" + service.name).CallFunc(func(g *jen.Group) {
					ifNamed(service.named, g, jen.Id("serviceName"))
					g.Id("c")
				})),
			)

		f.Line()

		// isVisited method
		f.Func().
			Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("isVisited" + service.name).
			ParamsFunc(ifNamedFunc(service.named, jen.Id("serviceName").String())).
			Bool().
			BlockFunc(func(g *jen.Group) {
				g.Id("c").Dot("visitLock").Dot("Lock").Call()
				g.Defer().Id("c").Dot("visitLock").Dot("Unlock").Call()

				g.Line()

				if service.named {
					g.Return(jen.Id("c").Dot("visited" + service.name).Index(jen.Id("serviceName")))
				} else {
					g.Return(jen.Id("c").Dot("visited" + service.name))
				}
			})

		f.Line()

		// markVisited method
		f.Func().
			Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("markVisited" + service.name).
			ParamsFunc(ifNamedFunc(service.named, jen.Id("serviceName").String())).
			BlockFunc(func(g *jen.Group) {
				g.Id("c").Dot("visitLock").Dot("Lock").Call()
				g.Defer().Id("c").Dot("visitLock").Dot("Unlock").Call()

				g.Line()

				if service.named {
					g.Id("c").Dot("visited" + service.name).Index(jen.Id("serviceName")).Op("=").Lit(true)
					g.Id("c").Dot("dependencyGraph").Op("=").Append(jen.Id("c").Dot("dependencyGraph"), jen.Lit(service.name+":").Op("+").Id("serviceName"))
				} else {
					g.Id("c").Dot("visited" + service.name).Op("=").Lit(true)
					g.Id("c").Dot("dependencyGraph").Op("=").Append(jen.Id("c").Dot("dependencyGraph"), jen.Lit(service.name))
				}
			})
	}
}

func generateCircularDependencyError(f *jen.File) {
	f.Comment("CircularDependencyError is returned when there is a circular dependency between two services.")
	f.Type().Id("CircularDependencyError").Struct(
		jen.Id("ServiceType").String(),
		jen.Id("ServiceName").String(),
		jen.Id("DependencyGraph").Index().String(),
	)

	f.Func().Id("newCircularDependencyError").Params(
		jen.Id("serviceType").String(),
		jen.Id("serviceName").String(),
		jen.Id("dependencyGraph").Index().String(),
	).Id("CircularDependencyError").Block(
		jen.Return(jen.Id("CircularDependencyError").Values(jen.Dict{
			jen.Id("ServiceType"):     jen.Id("serviceType"),
			jen.Id("ServiceName"):     jen.Id("serviceName"),
			jen.Id("DependencyGraph"): jen.Id("dependencyGraph"),
		})),
	)

	f.Func().Params(jen.Id("e").Id("CircularDependencyError")).Id("Error").Params().String().Block(
		jen.Id("dependencyPath").Op(":=").Qual("strings", "Join").Call(jen.Id("e").Dot("DependencyGraph"), jen.Lit(" -> ")),
		jen.Return(jen.Qual("fmt", "Sprintf").Call(
			jen.Lit("circular dependency detected for %s '%s': %s"),
			jen.Id("e").Dot("ServiceType"),
			jen.Id("e").Dot("ServiceName"),
			jen.Id("dependencyPath"),
		)),
	)
}

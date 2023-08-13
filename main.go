package main

import (
	"fmt"
	"os"

	"github.com/dave/jennifer/jen"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "insufficient arguments")
		fmt.Fprintln(os.Stderr, "usage: <command> PACKAGE SERVICE_TYPE [SERVICE_TYPE...]")
	}

	pkg := os.Args[1]
	services := os.Args[2:]

	f := jen.NewFile(pkg)
	f.ImportName("sync", "sync")
	f.ImportName("fmt", "fmt")
	f.ImportName("strings", "strings")

	generateServiceLocator(f, services)
	generateGenericNamedServiceFactory(f)
	generateServiceRegistry(f, services)
	generateServiceLocationContext(f, services)
	generateCircularDependencyError(f)

	err := f.Render(os.Stdout)
	if err != nil {
		fmt.Fprint(os.Stderr, "Error generating the code:", err)
		os.Exit(1)
	}
}

func generateServiceLocator(f *jen.File, services []string) {
	f.Comment("ServiceLocator locates named services in a type-safe manner.")
	f.Type().Id("ServiceLocator").InterfaceFunc(func(g *jen.Group) {
		for _, service := range services {
			g.Id("Get"+service).Params(jen.Id("name").String()).Params(jen.Id(service), jen.Error())
		}
	})
}

func generateGenericNamedServiceFactory(f *jen.File) {
	f.Comment("NamedServiceFactory creates a new named instance of T.")
	f.Type().Id("NamedServiceFactory").Types(jen.Id("T").Any()).Func().Params(jen.String(), jen.Id("ServiceLocator")).Params(jen.Id("T"), jen.Error())
}

func generateServiceRegistry(f *jen.File, services []string) {
	f.Comment("ServiceRegistry allows registering service factories to construct new instances of a service.")
	f.Comment("ServiceRegistry is also the primary {ServiceLocator} entrypoint.")
	f.Type().Id("ServiceRegistry").StructFunc(func(g *jen.Group) {
		g.Id("mu").Qual("sync", "Mutex")
		g.Line()

		for _, service := range services {
			g.Id("instances" + service).Map(jen.String()).Id(service)
			g.Id("factories" + service).Map(jen.String()).Id("NamedServiceFactory").Types(jen.Id(service))
		}
	})

	f.Comment("NewServiceRegistry instantiates a new {ServiceRegistry}.")
	f.Func().Id("NewServiceRegistry").Params().Op("*").Id("ServiceRegistry").Block(
		jen.Return(jen.Op("&").Id("ServiceRegistry").ValuesFunc(func(g *jen.Group) {
			for _, service := range services {
				g.Id("instances" + service).Op(":").Make(jen.Map(jen.String()).Id(service))
				g.Id("factories" + service).Op(":").Make(jen.Map(jen.String()).Id("NamedServiceFactory").Types(jen.Id(service)))
			}
		})),
	)

	generateServiceRegistryMethods(f, services)
}

func generateServiceRegistryMethods(f *jen.File, services []string) {
	for _, service := range services {
		f.Line()

		// Register method
		f.Commentf("Register%s registers a factory for {%s}.", service, service)
		f.Func().Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("Register"+service).Params(
			jen.Id("serviceName").String(),
			jen.Id("factory").Id("NamedServiceFactory").Types(jen.Id(service)),
		).Block(
			jen.Id("r").Dot("mu").Dot("Lock").Call(),
			jen.Defer().Id("r").Dot("mu").Dot("Unlock").Call(),
			jen.Line(),
			jen.Id("r").Dot("factories"+service).Index(jen.Id("serviceName")).Op("=").Id("factory"),
		)

		f.Line()

		// Get method
		f.Commentf("Get%s retrieves an instance of {%s}.", service, service)
		f.Func().Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("Get"+service).Params(
			jen.Id("serviceName").String(),
		).Params(jen.Id(service), jen.Error()).Block(
			jen.Return().Id("r").Dot("get"+service).Call(jen.Id("serviceName"), jen.Id("newServiceLocationContext").Call(jen.Id("r"), jen.Lit(0))),
		)

		f.Line()

		// Private get method
		f.Func().Params(jen.Id("r").Op("*").Id("ServiceRegistry")).Id("get"+service).Params(
			jen.Id("serviceName").String(),
			jen.Id("ctx").Op("*").Id("serviceLocationContext"),
		).Params(jen.Id(service), jen.Error()).BlockFunc(func(g *jen.Group) {
			g.Id("r").Dot("mu").Dot("Lock").Call()
			g.Id("instance, instanceOk").Op(":=").Id("r").Dot("instances" + service).Index(jen.Id("serviceName"))
			g.Id("factory, factoryOk").Op(":=").Id("r").Dot("factories" + service).Index(jen.Id("serviceName"))
			g.Id("r").Dot("mu").Dot("Unlock").Call()

			g.Line()

			g.If(jen.Id("instanceOk")).Block(
				jen.Return(jen.Id("instance"), jen.Nil()),
			)

			g.Line()

			g.If(jen.Id("ctx").Dot("isVisited" + service).Call(jen.Id("serviceName"))).Block(
				jen.Return(jen.Nil(), jen.Id("newCircularDependencyError").Call(
					jen.Lit(service),
					jen.Id("serviceName"),
					jen.Id("ctx").Dot("dependencyGraph"),
				)),
			)

			g.Id("ctx").Dot("markVisited" + service).Call(jen.Id("serviceName"))

			g.Line()

			g.If(jen.Op("!").Id("factoryOk")).Block(
				jen.Return(jen.Nil(), jen.Qual("fmt", "Errorf").Call(
					jen.Lit("no factory registered for "+service+" with name '%s'"),
					jen.Id("serviceName"),
				)),
			)

			g.Line()

			g.Id("instance, err").Op(":=").Id("factory").Call(jen.Id("serviceName"), jen.Id("ctx"))
			g.If(jen.Id("err").Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Id("err")),
			)

			g.Line()

			g.Id("r").Dot("mu").Dot("Lock").Call()
			g.Id("r").Dot("instances" + service).Index(jen.Id("serviceName")).Op("=").Id("instance")
			g.Id("r").Dot("mu").Dot("Unlock").Call()

			g.Line()

			g.Return(jen.Id("instance"), jen.Nil())
		})
	}
}

func generateServiceLocationContext(f *jen.File, services []string) {
	f.Type().Id("serviceLocationContext").StructFunc(func(g *jen.Group) {
		g.Id("registry").Op("*").Id("ServiceRegistry")
		g.Id("dependencyGraph").Index().String()

		g.Line()

		g.Id("visitLock").Qual("sync", "Mutex")

		g.Line()

		for _, service := range services {
			g.Id("visited" + service).Map(jen.String()).Bool()
		}
	})

	f.Func().Id("newServiceLocationContext").Params(jen.Id("registry").Op("*").Id("ServiceRegistry"), jen.Id("maxDepth").Int()).Op("*").Id("serviceLocationContext").Block(
		jen.Return(jen.Op("&").Id("serviceLocationContext").ValuesFunc(func(g *jen.Group) {
			g.Id("registry").Op(":").Id("registry")

			for _, service := range services {
				g.Id("visited" + service).Op(":").Make(jen.Map(jen.String()).Bool())
			}
		})),
	)

	for _, iface := range services {
		f.Line()

		// Get method
		f.Func().Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("Get"+iface).Params(jen.Id("serviceName").String()).Params(jen.Id(iface), jen.Error()).Block(
			jen.Return(jen.Id("c").Dot("registry").Dot("get"+iface).Call(jen.Id("serviceName"), jen.Id("c"))),
		)

		f.Line()

		// isVisited method
		f.Func().Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("isVisited"+iface).Params(jen.Id("serviceName").String()).Bool().Block(
			jen.Id("c").Dot("visitLock").Dot("Lock").Call(),
			jen.Defer().Id("c").Dot("visitLock").Dot("Unlock").Call(),

			jen.Line(),

			jen.Return(jen.Id("c").Dot("visited"+iface).Index(jen.Id("serviceName"))),
		)

		f.Line()

		// markVisited method
		f.Func().Params(jen.Id("c").Op("*").Id("serviceLocationContext")).Id("markVisited"+iface).Params(jen.Id("serviceName").String()).Block(
			jen.Id("c").Dot("visitLock").Dot("Lock").Call(),
			jen.Defer().Id("c").Dot("visitLock").Dot("Unlock").Call(),

			jen.Line(),

			jen.Id("c").Dot("visited"+iface).Index(jen.Id("serviceName")).Op("=").Lit(true),
			jen.Id("c").Dot("dependencyGraph").Op("=").Append(jen.Id("c").Dot("dependencyGraph"), jen.Lit(iface+":").Op("+").Id("serviceName")),
		)
	}
}

func generateCircularDependencyError(f *jen.File) {
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

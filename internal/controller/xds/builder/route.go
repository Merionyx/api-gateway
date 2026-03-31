// internal/xds/builder/route.go
package builder

import (
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"merionyx/api-gateway/internal/controller/domain/models"
)

func (b *xdsBuilder) BuildRoutes(env *models.Environment) []*routev3.RouteConfiguration {
	routes := make([]*routev3.Route, 0)

	for _, snapshot := range env.Snapshots {
		route := &routev3.Route{
			Name: snapshot.Name,
			Match: &routev3.RouteMatch{
				PathSpecifier: &routev3.RouteMatch_Prefix{
					Prefix: snapshot.Prefix,
				},
			},
			Action: &routev3.Route_Route{
				Route: &routev3.RouteAction{
					ClusterSpecifier: &routev3.RouteAction_Cluster{
						Cluster: snapshot.Upstream.Name,
					},
					PrefixRewrite: "/",
				},
			},
		}
		routes = append(routes, route)
	}

	routeConfig := &routev3.RouteConfiguration{
		Name: env.Name + "_routes",
		VirtualHosts: []*routev3.VirtualHost{{
			Name:    env.Name + "_vhost",
			Domains: []string{"*"},
			Routes:  routes,
		}},
	}

	return []*routev3.RouteConfiguration{routeConfig}
}

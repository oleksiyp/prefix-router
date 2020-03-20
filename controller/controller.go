package controller

import (
	consulapi "github.com/hashicorp/consul/api"
	"github.com/oleksiyp/prefixrouter/pkg/apis/prefixrouter/v1beta1"
	"github.com/oleksiyp/prefixrouter/pkg/client/clientset/versioned"
	informer "github.com/oleksiyp/prefixrouter/pkg/client/informers/externalversions/prefixrouter/v1beta1"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	serviceName        string
	kubeClient         kubernetes.Interface
	prefixRouterClient *versioned.Clientset
	consulClient       *consulapi.Client
	routeInformer      informer.RouteInformer
	logger             *zap.SugaredLogger
	operations         <-chan RouteOperation
	routes             map[string]string
}

type RouteOperation struct {
	Add   bool
	Route v1beta1.Route
}

func (c Controller) Run(stopCh <-chan struct{}) error {
	for {
		select {
		case op := <-c.operations:
			if op.Add {
				c.routes[op.Route.Spec.Prefix] = op.Route.Spec.Service
			} else {
				delete(c.routes, op.Route.Spec.Prefix)
			}

			c.refreshRoutes()
		case <-stopCh:
			return nil
		}
	}
}

func NewController(
	serviceName string,
	kubeClient kubernetes.Interface,
	prefixRouterClient *versioned.Clientset,
	consulClient *consulapi.Client,
	routeInformer informer.RouteInformer,
	logger *zap.SugaredLogger,
) *Controller {
	operations := make(chan RouteOperation)

	controller := &Controller{
		serviceName,
		kubeClient,
		prefixRouterClient,
		consulClient,
		routeInformer,
		logger,
		operations,
		make(map[string]string),
	}

	routeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			route, ok := checkCustomResourceType(obj, logger)
			if !ok {
				return
			}

			logger.Info("Adding route ", route.Spec.Prefix, " -> ", route.Spec.Service)
			operations <- RouteOperation{
				Add:   true,
				Route: route,
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			route, ok := checkCustomResourceType(newObj, logger)
			if !ok {
				return
			}

			logger.Info("Updating route ", route.Spec.Prefix, " -> ", route.Spec.Service)
			operations <- RouteOperation{
				Add:   true,
				Route: route,
			}
		},
		DeleteFunc: func(obj interface{}) {
			route, ok := checkCustomResourceType(obj, logger)
			if !ok {
				return
			}

			logger.Info("Deleting route ", route.Spec.Prefix, " -> ", route.Spec.Service)
			operations <- RouteOperation{
				Add:   false,
				Route: route,
			}
		},
	})

	return controller
}

func (c Controller) refreshRoutes() {
	configEntry := consulapi.ServiceRouterConfigEntry{
		Kind:      consulapi.ServiceRouter,
		Name:      c.serviceName,
		Namespace: "",
		Routes:    []consulapi.ServiceRoute{},
	}

	for prefix, serviceName := range c.routes {
		configEntry.Routes = append(configEntry.Routes, consulapi.ServiceRoute{
			Match: &consulapi.ServiceRouteMatch{
				HTTP: &consulapi.ServiceRouteHTTPMatch{
					PathPrefix: prefix,
				},
			},
			Destination: &consulapi.ServiceRouteDestination{
				Service: serviceName,
			},
		})
	}

	ok, _, err := c.consulClient.ConfigEntries().Set(&configEntry, nil)

	if err != nil {
		c.logger.Errorf("Failed to reconfigure consul: %#v", err)
		return
	}
	if !ok {
		c.logger.Errorf("Failed to reconfigure consul: HTTP request returned not 'true'")
		return
	}
}

func checkCustomResourceType(obj interface{}, logger *zap.SugaredLogger) (v1beta1.Route, bool) {
	var roll *v1beta1.Route
	var ok bool
	if roll, ok = obj.(*v1beta1.Route); !ok {
		logger.Errorf("Event Watch received an invalid object: %#v", obj)
		return v1beta1.Route{}, false
	}
	return *roll, true
}

func checkSplitterConfigEntry(obj consulapi.ConfigEntry, logger *zap.SugaredLogger) (consulapi.ServiceSplitterConfigEntry, bool) {
	var roll *consulapi.ServiceSplitterConfigEntry
	var ok bool
	if roll, ok = obj.(*consulapi.ServiceSplitterConfigEntry); !ok {
		logger.Errorf("Watch received an invalid object: %#v", obj)
		return consulapi.ServiceSplitterConfigEntry{}, false
	}
	return *roll, true
}

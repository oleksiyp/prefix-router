package controller

import (
	consulapi "github.com/hashicorp/consul/api"
	"github.com/oleksiyp/prefixrouter/pkg/apis/prefixrouter/v1beta1"
	"github.com/oleksiyp/prefixrouter/pkg/client/clientset/versioned"
	informer "github.com/oleksiyp/prefixrouter/pkg/client/informers/externalversions/prefixrouter/v1beta1"
	"github.com/oleksiyp/prefixrouter/watcher"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type RouteInfo struct {
	watcher     *watcher.ConfigEntryWatcher
	serviceName string
}

type Controller struct {
	serviceName        string
	kubeClient         kubernetes.Interface
	prefixRouterClient *versioned.Clientset
	consulClient       *consulapi.Client
	routeInformer      informer.RouteInformer
	logger             *zap.SugaredLogger
	operations         <-chan RouteOperation
	routes             map[string]RouteInfo
}

type RouteOperation struct {
	Add   bool
	Route v1beta1.Route
}

func (c Controller) Run(stopCh <-chan struct{}) error {
	configEntries := make(chan consulapi.ConfigEntry)
	for {
		select {
		case entry := <-configEntries:
			splitterEntry, ok := checkSplitterConfigEntry(entry, c.logger)
			if ok {
				splitterEntry.Name := c.serviceName
				splitterEntry.Splits = append(splitterEntry.Splits, )
			}
		case op := <-c.operations:
			if op.Add {
				configEntryWatcher := watcher.NewConfigEntryWatcher(
					c.consulClient,
					consulapi.ServiceSplitter,
					op.Route.Spec.Service,
				)

				err := configEntryWatcher.Watch(configEntries)
				if err != nil {
					c.logger.Warn("Failed to watch splitter: %v", err)
				}

				c.routes[op.Route.Spec.Prefix] = RouteInfo{
					watcher:     configEntryWatcher,
					serviceName: op.Route.Spec.Service,
				}
			} else {
				info, ok := c.routes[op.Route.Spec.Prefix]
				if ok {
					info.watcher.Cancel()
					delete(c.routes, op.Route.Spec.Prefix)
				}
			}

			c.refreshRoutes()
		case <-stopCh:
			for _, v := range c.routes {
				v.watcher.Cancel()
			}
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
		make(map[string]RouteInfo),
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

	for prefix, routeInfo := range c.routes {
		configEntry.Routes = append(configEntry.Routes, consulapi.ServiceRoute{
			Match: &consulapi.ServiceRouteMatch{
				HTTP: &consulapi.ServiceRouteHTTPMatch{
					PathPrefix: prefix,
				},
			},
			Destination: &consulapi.ServiceRouteDestination{
				Service: routeInfo.serviceName,
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

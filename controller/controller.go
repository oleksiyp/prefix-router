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
	kubeClient         kubernetes.Interface
	prefixRouterClient *versioned.Clientset
	consulClient       *consulapi.Client
	routeInformer      informer.RouteInformer
	logger             *zap.SugaredLogger
}

func (c Controller) Run(stopCh <-chan struct{}) error {
	for {
		select {
		case <-stopCh:
			return nil
		}
	}
}

func NewController(
	kubeClient kubernetes.Interface,
	prefixRouterClient *versioned.Clientset,
	consulClient *consulapi.Client,
	routeInformer informer.RouteInformer,
	logger *zap.SugaredLogger,
) *Controller {

	controller := &Controller{
		kubeClient,
		prefixRouterClient,
		consulClient,
		routeInformer,
		logger,
	}

	routeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			route, ok := checkCustomResourceType(obj, logger)
			if !ok {
				return
			}

			logger.Info("Added ", route.Spec.Prefix)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			route, ok := checkCustomResourceType(newObj, logger)
			if !ok {
				return
			}

			logger.Info("Update ", route.Spec.Prefix)
		},
		DeleteFunc: func(obj interface{}) {
			route, ok := checkCustomResourceType(obj, logger)
			if !ok {
				return
			}

			logger.Info("Deleted ", route.Spec.Prefix)
		},
	})

	return controller
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

package main

import (
	"flag"
	semver "github.com/Masterminds/semver/v3"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/oleksiyp/prefixrouter/controller"
	clientset "github.com/oleksiyp/prefixrouter/pkg/client/clientset/versioned"
	"github.com/oleksiyp/prefixrouter/pkg/client/informers/externalversions"
	"github.com/oleksiyp/prefixrouter/pkg/client/informers/externalversions/prefixrouter/v1beta1"
	"github.com/oleksiyp/prefixrouter/pkg/logger"
	"github.com/oleksiyp/prefixrouter/pkg/signals"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/code-generator/cmd/client-gen/generators"
	"log"
	"time"
)

var (
	masterURL   string
	kubeconfig  string
	logLevel    string
	zapEncoding string
	namespace   string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&logLevel, "log-level", "debug", "Log level can be: debug, info, warning, error.")
	flag.StringVar(&zapEncoding, "zap-encoding", "json", "Zap logger encoding.")
	flag.StringVar(&namespace, "namespace", "", "Namespace that flagger would watch canary object.")
}

func main() {
	flag.Parse()

	logger, err := logger.NewLoggerWithEncoding(logLevel, zapEncoding)
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}

	defer logger.Sync()

	stopCh := signals.SetupSignalHandler()

	logger.Infof("Starting prefixrouter")

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	prefixRouterClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building prefix router clientset: %v", err)
	}

	consulClient, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		logger.Fatalf("Error building consul client: %v", err)
	}

	verifyCRDs(prefixRouterClient, logger)
	verifyKubernetesVersion(kubeClient, logger)
	verifyConsulClient(*consulClient, logger)

	routeInformer := startInformers(prefixRouterClient, logger, stopCh)

	c := controller.NewController(
		kubeClient,
		prefixRouterClient,
		consulClient,
		routeInformer,
		logger,
	)

	if err := c.Run(stopCh); err != nil {
		logger.Fatalf("Error running controller: %v", err)
	}
}

func startInformers(
	client *clientset.Clientset,
	logger *zap.SugaredLogger,
	stopCh <-chan struct{},
) v1beta1.RouteInformer {
	informerFactory := externalversions.NewSharedInformerFactoryWithOptions(client, time.Second*30, externalversions.WithNamespace(namespace))

	logger.Info("Waiting for route informer cache to sync")
	routeInformer := informerFactory.Prefixrouter().V1beta1().Routes()
	go routeInformer.Informer().Run(stopCh)
	if ok := cache.WaitForNamedCacheSync("prefixrouter", stopCh, routeInformer.Informer().HasSynced); !ok {
		logger.Fatalf("failed to wait for cache to sync")
	}

	return routeInformer
}

func verifyCRDs(client clientset.Interface, logger *zap.SugaredLogger) {
	_, err := client.PrefixrouterV1beta1().Routes(namespace).List(metav1.ListOptions{Limit: 1})
	if err != nil {
		logger.Fatalf("Route CRD is not registered %v", err)
	}
}

func verifyKubernetesVersion(kubeClient kubernetes.Interface, logger *zap.SugaredLogger) {
	ver, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		logger.Fatalf("Error calling Kubernetes API: %v", err)
	}

	k8sVersionConstraint := "^1.11.0"

	// We append -alpha.1 to the end of our version constraint so that prebuilds of later versions
	// are considered valid for our purposes, as well as some managed solutions like EKS where they provide
	// a version like `v1.12.6-eks-d69f1b`. It doesn't matter what the prelease value is here, just that it
	// exists in our constraint.
	semverConstraint, err := semver.NewConstraint(k8sVersionConstraint + "-alpha.1")
	if err != nil {
		logger.Fatalf("Error parsing kubernetes version constraint: %v", err)
	}

	k8sSemver, err := semver.NewVersion(ver.GitVersion)
	if err != nil {
		logger.Fatalf("Error parsing kubernetes version as a semantic version: %v", err)
	}

	if !semverConstraint.Check(k8sSemver) {
		logger.Fatalf("Unsupported version of kubernetes detected.  Expected %s, got %v", k8sVersionConstraint, ver)
	}

	logger.Infof("Connected to Kubernetes API %s", ver)
}

func verifyConsulClient(client consulapi.Client, logger *zap.SugaredLogger) {
	name, err := client.Agent().NodeName()
	if err != nil {
		logger.Fatalf("Consul not reachable %v", err)
	}

	logger.Infof("Connected to Consul API, agent node = %s", name)
}

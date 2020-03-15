package main

import (
	"flag"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/oleksiyp/multi-dc-final/prefix-router/pkg/logger"
	"github.com/oleksiyp/multi-dc-final/prefix-router/pkg/signals"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/code-generator/cmd/client-gen/generators"
	"log"
)

var (
	masterURL   string
	kubeconfig  string
	logLevel    string
	zapEncoding string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&logLevel, "log-level", "debug", "Log level can be: debug, info, warning, error.")
	flag.StringVar(&zapEncoding, "zap-encoding", "json", "Zap logger encoding.")
}

func main() {
	logger, err := logger.NewLoggerWithEncoding(logLevel, zapEncoding)
	if err != nil {
		log.Fatalf("Error creating logger: %v", err)
	}

	defer logger.Sync()

	stopCh := signals.SetupSignalHandler()

	logger.Infof("Starting prefix-router")

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	consulClient, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		logger.Fatalf("Error building consul client: %v", err)
	}



}

func verifyCRDs(flaggerClient clientset.Interface, logger *zap.SugaredLogger) {
	_, err := flaggerClient.FlaggerV1beta1().Canaries(namespace).List(metav1.ListOptions{Limit: 1})
	if err != nil {
		logger.Fatalf("Canary CRD is not registered %v", err)
	}

	_, err = flaggerClient.FlaggerV1beta1().MetricTemplates(namespace).List(metav1.ListOptions{Limit: 1})
	if err != nil {
		logger.Fatalf("MetricTemplate CRD is not registered %v", err)
	}

	_, err = flaggerClient.FlaggerV1beta1().AlertProviders(namespace).List(metav1.ListOptions{Limit: 1})
	if err != nil {
		logger.Fatalf("AlertProvider CRD is not registered %v", err)
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

package main

import (
	"context"
	controller "github.com/BrunoSilvaFreire/homelab-router-operator/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"log"
	"os"
	"os/signal"
)

func main() {
	logger := log.Default()
	level := klog.Level(6)
	err := level.Set("6")
	if err != nil {
		logger.Printf("Failed to set klog level: %s", err.Error())
		return
	}
	config, success := loadConfig(logger)
	if !success {
		logger.Printf("Failed to load kubernetes config")
		return
	}

	endpoint, found := os.LookupEnv("ROUTER_OVERRIDE_KUBERNETES_ENDPOINT")

	if found {
		logger.Printf("Overriding Kubernetes endpoint to %s", endpoint)
		config.Host = endpoint
	}

	logger.Printf("Final kubernetes config: %v", config.String())

	kube := kubernetes.NewForConfigOrDie(config)

	URL, found := os.LookupEnv("ROUTER_URL")
	if !found {
		logger.Printf("ROUTER_URL not found")
		return
	}

	username, found := os.LookupEnv("ROUTER_USERNAME")
	if !found {
		logger.Printf("ROUTER_USERNAME not found")
		return
	}

	password, found := os.LookupEnv("ROUTER_PASSWORD")
	if !found {
		logger.Printf("ROUTER_PASSWORD not found")
		return
	}

	hostFormat, found := os.LookupEnv("ROUTER_SERVICE_HOST_FORMAT")
	if !found {
		logger.Printf("ROUTER_SERVICE_HOST_FORMAT not found")
		return
	}

	listenToIngresses := true
	listenToLoadBalancers := true

	ignoredIngress, found := os.LookupEnv("ROUTER_SERVICE_NO_INGRESSES")
	if ignoredIngress == "true" || ignoredIngress == "1" {
		listenToIngresses = false
	}
	ignoredLoadBalancers, found := os.LookupEnv("ROUTER_SERVICE_NO_LOADBALANCERS")
	if ignoredLoadBalancers == "true" || ignoredLoadBalancers == "1" {
		listenToLoadBalancers = false
	}

	routerController := controller.CreateRouterController(URL, username, password, hostFormat, kube, listenToIngresses, listenToLoadBalancers)
	go func() {
		// Watch for sigint
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		routerController.Stop()
	}()
	err = routerController.Run(context.Background())
	if err != nil {
		logger.Fatalf("Error running controller: %s", err.Error())
		return
	}
}

func loadConfig(logger *log.Logger) (*rest.Config, bool) {
	config, err := rest.InClusterConfig()
	if err == nil {
		logger.Printf("Using in-cluster config")
		return config, true
	}
	// Try load credentials from .kube/config
	kubeLocation := "~/.kube/config"
	env, found := os.LookupEnv("ROUTER_KUBECONFIG_PATH")
	if found {
		kubeLocation = env
	}
	config, err = clientcmd.BuildConfigFromFlags("", kubeLocation)
	if err == nil {
		logger.Printf("Using config from kubeconfig file at %s", kubeLocation)
		return config, true
	}
	logger.Printf("Unable to load configuration.")
	return nil, false
}

package main

import (
	"context"
	controller "github.com/BrunoSilvaFreire/homelab-router-operator/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"os/signal"
)

func main() {
	logger := log.Default()
	config, success := loadConfig(logger)
	if !success {
		logger.Printf("Failed to load kubernetes config")
		return
	}

	logger.Printf("Kubernetes config: %v", config.String())

	kube := kubernetes.NewForConfigOrDie(config)
	URL, found := os.LookupEnv("ROUTER_URL")
	if !found {
		logger.Printf("ROUTER_URL not found")
		return
	}
	Username, found := os.LookupEnv("ROUTER_USERNAME")
	if !found {
		logger.Printf("ROUTER_USERNAME not found")
		return
	}
	Password, found := os.LookupEnv("ROUTER_PASSWORD")
	if !found {
		logger.Printf("ROUTER_PASSWORD not found")
		return
	}
	routerController := controller.CreateRouterController(URL, Username, Password, kube)
	go func() {
		// Watch for sigint
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		routerController.Stop()
	}()
	err := routerController.Run(context.Background())
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

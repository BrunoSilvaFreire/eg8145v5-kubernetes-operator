package controller

import (
	"context"
	"github.com/chickenzord/go-huawei-client/pkg/eg8145v5"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"log"
	"reflect"
	"slices"
)

type RouterIngressController struct {
	logger           *log.Logger
	routerClient     *eg8145v5.Client
	kubernetesClient kubernetes.Interface
	currentWatcher   watch.Interface
}
type Clientset struct {
	ingress *v1.Ingress
}

func CreateRouterController(
	URL string,
	Username string,
	Password string,
	KubernetesClient kubernetes.Interface,
) RouterIngressController {
	return RouterIngressController{
		logger:           log.Default(),
		kubernetesClient: KubernetesClient,
		routerClient: eg8145v5.NewClient(eg8145v5.Config{
			URL:      URL,
			Username: Username,
			Password: Password,
		}),
	}
}
func (c *RouterIngressController) loop(ctx context.Context, channel chan<- error) {
	c.logger.Printf("Starting ingresses watch")
	watcher, err := c.kubernetesClient.NetworkingV1().Ingresses("").Watch(ctx, metav1.ListOptions{})
	c.currentWatcher = watcher
	if err != nil {
		c.logger.Printf("Failed to watch ingresses: %s", err.Error())
		channel <- err
		return
	}

	for event := range watcher.ResultChan() {
		ingress, ok := event.Object.(*v1.Ingress)
		if !ok {
			c.logger.Printf("Received event %s with not ingress object: %s", event.Type, reflect.TypeOf(event.Object).Name())
			continue
		}
		c.logger.Printf("Received event %s for ingress %s", event.Type, ingress.Name)
		err := c.syncIngress(ingress)
		if err != nil {
			channel <- err
			return
		}
	}
	c.logger.Printf("Finished watching ingresses")
	channel <- nil
}

func (c *RouterIngressController) Run(ctx context.Context) error {
	channel := make(chan error)
	go c.loop(ctx, channel)
	err := c.routerClient.Login()
	if err != nil {
		return err
	}

	err = <-channel
	if err != nil {
		return err
	}
	return nil
}

func (c *RouterIngressController) Stop() {
	c.logger.Print("Stopping controller")
	if c.currentWatcher != nil {
		c.currentWatcher.Stop()
	}
}

func (c *RouterIngressController) syncIngress(ingress *v1.Ingress) error {
	hosts, err := c.routerClient.GetAllStaticDnsHosts()
	if err != nil {
		err := c.routerClient.Login()
		if err != nil {
			return err
		}
		hosts, err = c.routerClient.GetAllStaticDnsHosts()
		if err != nil {
			return err
		}
	}
	for _, status := range ingress.Status.LoadBalancer.Ingress {
		for _, rule := range ingress.Spec.Rules {

			host := eg8145v5.StaticDnsHost{
				DomainName: rule.Host,
				IPAddress:  status.IP,
			}

			dnsEntryIndex := slices.IndexFunc(hosts, func(host eg8145v5.StaticDnsHost) bool {
				return rule.Host == host.DomainName
			})
			exists := dnsEntryIndex != -1
			if exists {
				dnsHost := hosts[dnsEntryIndex]
				if dnsHost.IPAddress == host.IPAddress {
					c.logger.Printf("Host %s up to date with ip %s", host.DomainName, host.IPAddress)
					continue
				}
				c.logger.Printf("Updating existing host %s on index %d to ip %s", host.DomainName, dnsEntryIndex, host.IPAddress)
				err := c.routerClient.SetDnsHost(host, dnsEntryIndex)
				if err != nil {
					return err
				}
			} else {
				// Create
				c.logger.Printf("Creating new host %s with ip %s", host.DomainName, host.IPAddress)
				err := c.routerClient.AddDnsHost(host)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

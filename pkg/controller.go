package controller

import (
	"context"
	"github.com/chickenzord/go-huawei-client/pkg/eg8145v5"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"log"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type RouterIngressController struct {
	logger                 *log.Logger
	routerClient           *eg8145v5.Client
	kubernetesClient       kubernetes.Interface
	ingressWatcher         watch.Interface
	loadBalancerWatcher    watch.Interface
	loadBalancerHostFormat string
	lastIps                map[string]string
	listenToIngresses      bool
	listenToLoadBalancers  bool
}
type Clientset struct {
	ingress *networkingv1.Ingress
}

func CreateRouterController(
	URL string,
	Username string,
	Password string,
	LoadBalancerHostFormat string,
	KubernetesClient kubernetes.Interface,
	ListenToIngresses bool,
	ListenToLoadBalancers bool,
) RouterIngressController {
	return RouterIngressController{
		logger:                 log.Default(),
		kubernetesClient:       KubernetesClient,
		loadBalancerHostFormat: LoadBalancerHostFormat,
		lastIps:                map[string]string{},
		listenToIngresses:      ListenToIngresses,
		listenToLoadBalancers:  ListenToLoadBalancers,
		routerClient: eg8145v5.NewClient(eg8145v5.Config{
			URL:      URL,
			Username: Username,
			Password: Password,
		}),
	}
}

func (c *RouterIngressController) ingressWatch(ctx context.Context, waitGroup *sync.WaitGroup, channel chan<- error) {
	defer waitGroup.Done()

	c.logger.Printf("Starting ingresses watch")
	watcher, err := c.kubernetesClient.NetworkingV1().Ingresses("").Watch(ctx, metav1.ListOptions{})
	c.ingressWatcher = watcher
	if err != nil {
		c.logger.Printf("Failed to watch ingresses: %s", err.Error())
		channel <- err
		return
	}

	for event := range watcher.ResultChan() {
		ingress, ok := event.Object.(*networkingv1.Ingress)
		if !ok {
			c.logger.Printf("Received event %s with not ingress object: %s", event.Type, reflect.TypeOf(event.Object).Name())
			continue
		}
		c.logger.Printf("Received event %s for ingress %s", event.Type, ingress.Name)

		var hosts []eg8145v5.StaticDnsHost
		for _, lbIngress := range ingress.Status.LoadBalancer.Ingress {

			for _, rule := range ingress.Spec.Rules {

				hosts = append(hosts, eg8145v5.StaticDnsHost{
					DomainName: rule.Host,
					IPAddress:  lbIngress.IP,
				})
			}
		}

		err := c.syncHosts(hosts)
		if err != nil {
			channel <- err
			return
		}
	}
	c.logger.Printf("Finished watching ingresses")
	channel <- nil
}

const nameLabel = "app.kubernetes.io/name"

func selectServiceName(service *corev1.Service) string {
	s := service.Labels[nameLabel]
	if len(s) > 0 {
		return s
	}
	return service.Name
}
func (c *RouterIngressController) loadBalancerWatch(ctx context.Context, waitGroup *sync.WaitGroup, channel chan error) {
	defer waitGroup.Done()

	c.logger.Printf("Starting loadBalancer watch")
	watcher, err := c.kubernetesClient.CoreV1().Services("").Watch(ctx, metav1.ListOptions{})
	c.ingressWatcher = watcher
	if err != nil {
		c.logger.Printf("Failed to watch ingresses: %s", err.Error())
		channel <- err
		return
	}

	for event := range watcher.ResultChan() {
		service, ok := event.Object.(*corev1.Service)

		if !ok {
			c.logger.Printf("Received event %s with not service object: %s", event.Type, reflect.TypeOf(event.Object).Name())
			continue
		}

		if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		name := selectServiceName(service)
		host := strings.Replace(c.loadBalancerHostFormat, "{name}", name, -1)

		var hosts []eg8145v5.StaticDnsHost

		for _, lbIngress := range service.Status.LoadBalancer.Ingress {
			previous := c.lastIps[host]
			if len(previous) > 0 && previous == lbIngress.IP {
				continue
			}

			hosts = append(hosts, eg8145v5.StaticDnsHost{
				DomainName: host,
				IPAddress:  lbIngress.IP,
			})
			c.lastIps[host] = lbIngress.IP
		}
		if len(hosts) > 0 {
			c.logger.Printf("Reacting to event %s for service %s", event.Type, service.Name)
			err := c.syncHosts(hosts)
			if err != nil {
				channel <- err
				return
			}
		}
	}
	c.logger.Printf("Finished watching loadBalancers")
	channel <- nil
}

func (c *RouterIngressController) Run(ctx context.Context) error {
	group := sync.WaitGroup{}
	jobCount := 0
	if c.listenToIngresses {
		jobCount++
	}
	if c.listenToLoadBalancers {
		jobCount++
	}
	group.Add(jobCount)
	channel := make(chan error, jobCount)

	if c.listenToIngresses {
		go c.ingressWatch(ctx, &group, channel)
	}

	if c.listenToLoadBalancers {
		go c.loadBalancerWatch(ctx, &group, channel)
	}
	err := c.routerClient.Login()
	if err != nil {
		return err
	}

	c.logger.Printf("Controller started.")
	group.Wait()
	c.logger.Printf("Controller finishing execution.")
	c.logger.Printf("Checking for errors...")
	err = <-channel
	if err != nil {
		c.logger.Printf("Error: %s", err.Error())
		return err
	}
	c.logger.Printf("Controller finished execution.")
	return nil
}

func (c *RouterIngressController) Stop() {
	c.logger.Print("Stopping controller")
	if c.ingressWatcher != nil {
		c.ingressWatcher.Stop()
	}
	if c.loadBalancerWatcher != nil {
		c.loadBalancerWatcher.Stop()
	}
}

func (c *RouterIngressController) syncHosts(hosts []eg8145v5.StaticDnsHost) error {
	if len(hosts) == 0 {
		return nil
	}

	existingHosts, err := c.routerClient.GetAllStaticDnsHosts()
	if err != nil {
		err := c.routerClient.Login()
		if err != nil {
			return err
		}
		existingHosts, err = c.routerClient.GetAllStaticDnsHosts()
		if err != nil {
			return err
		}
	}

	for _, host := range hosts {
		dnsEntryIndex := slices.IndexFunc(existingHosts, func(other eg8145v5.StaticDnsHost) bool {
			return host.DomainName == other.DomainName
		})
		exists := dnsEntryIndex != -1
		if exists {
			// Update
			dnsHost := existingHosts[dnsEntryIndex]
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
	return nil
}

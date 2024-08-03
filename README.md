# eg8145v5-kubernetes-operator

Do you have one of these?  
<img src="assets/router.png" height=320px>

This operator watches Kubernetes ingresses and load balancers for changes and updates the router's static DNS
configuration. This allows you to automatically access your services externally.

## Configuration

These are all the environment variable the operator uses:

| Parameter                             | Description                                                                                                                                                              | Example               |
|---------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------|
| `ROUTER_URL`                          | The URL of your router. This is set as an environment variable in the `Deployment`.                                                                                      | `http://192.168.18.1` |
| `ROUTER_USERNAME`                     | The username of the admin for your router                                                                                                                                | unspecified           | 
| `ROUTER_PASSWORD`                     | The password of the admin for your router                                                                                                                                | unspecified           | 
| `ROUTER_SERVICE_HOST_FORMAT`          | The format that will be used                                                                                                                                             | {name}.leroy.lab      | 
| `ROUTER_SERVICE_NO_INGRESSES`         | If specified as "true" or "1", will not watch for ingresses                                                                                                              | false                 | 
| `ROUTER_SERVICE_NO_LOADBALANCERS`     | If specified as "true" or "1", will not watch for load balancers.                                                                                                        | false                 | 
| `ROUTER_OVERRIDE_KUBERNETES_ENDPOINT` | If specified, will override the endpoint used for the kubernetes API when using in cluster configuration                                                                 |                       | 
| `ROUTER_KUBECONFIG_PATH`              | If specified, when running outside of a cluster configuration (for development for example), will point to the configuration to use for connecting to the kubernetes api | ~/.kube/config        | 

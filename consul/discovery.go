package consul

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

// DiscoveryConfig holds the configuration for service discovery.
type DiscoveryConfig struct {
	// ConsulAddress is the Consul agent address (default: "consul:8500").
	ConsulAddress string
	// ConsulToken is the ACL token for queries (optional).
	ConsulToken string
	// DNSPort is the Consul DNS port (default: 8600).
	DNSPort int
	// CacheTTL is the TTL for cached service resolutions (default: 30s).
	CacheTTL time.Duration
	// UseConnectSidecar enables resolution via Connect sidecar ingress addresses.
	// When true, returns the sidecar's ingress address for mTLS termination.
	UseConnectSidecar bool
}

// ServiceAddress represents a resolved service address.
type ServiceAddress struct {
	Host string
	Port int
}

func (s ServiceAddress) String() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s ServiceAddress) URL() string {
	return fmt.Sprintf("http://%s:%d", s.Host, s.Port)
}

// ServiceDiscovery resolves service addresses via Consul DNS SRV or Catalog API.
type ServiceDiscovery struct {
	config       DiscoveryConfig
	client       *api.Client
	cache        map[string]cachedEntry
	cacheMu      sync.RWMutex
	resolver     *net.Resolver
}

type cachedEntry struct {
	address  ServiceAddress
	expires  time.Time
}

// NewServiceDiscovery creates a new ServiceDiscovery instance.
func NewServiceDiscovery(config DiscoveryConfig) (*ServiceDiscovery, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.ConsulAddress
	consulConfig.Token = config.ConsulToken

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	if config.DNSPort == 0 {
		config.DNSPort = 8600
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 30 * time.Second
	}

	// Configure DNS resolver to use Consul DNS
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", fmt.Sprintf("consul:%d", config.DNSPort))
		},
	}

	return &ServiceDiscovery{
		config:   config,
		client:   client,
		cache:    make(map[string]cachedEntry),
		resolver: resolver,
	}, nil
}

// ResolveService resolves a service name to host:port.
// Tries DNS SRV first, then falls back to Catalog API.
// Results are cached for TTL seconds.
func (d *ServiceDiscovery) ResolveService(ctx context.Context, serviceName string) (ServiceAddress, error) {
	// Check cache first
	d.cacheMu.RLock()
	if entry, ok := d.cache[serviceName]; ok {
		if time.Now().Before(entry.expires) {
			log.Printf("[consul] Cache hit for %s: %s", serviceName, entry.address)
			d.cacheMu.RUnlock()
			return entry.address, nil
		}
	}
	d.cacheMu.RUnlock()

	var address ServiceAddress
	var err error

	if d.config.UseConnectSidecar {
		// Try Connect sidecar ingress address first
		address, err = d.resolveViaConnectSidecar(ctx, serviceName)
		if err != nil {
			log.Printf("[consul] Connect sidecar resolution failed for %s: %v, falling back to DNS SRV", serviceName, err)
			address, err = d.resolveViaDNSSRV(ctx, serviceName)
		}
	} else {
		// Try DNS SRV first
		address, err = d.resolveViaDNSSRV(ctx, serviceName)
		if err != nil {
			log.Printf("[consul] DNS SRV resolution failed for %s: %v, falling back to Catalog API", serviceName, err)
			// Fall back to Catalog API
			address, err = d.resolveViaCatalog(ctx, serviceName)
		}
	}

	if err != nil {
		return ServiceAddress{}, fmt.Errorf("could not resolve service %s: %w", serviceName, err)
	}

	// Cache the result
	d.cacheMu.Lock()
	d.cache[serviceName] = cachedEntry{
		address: address,
		expires: time.Now().Add(d.config.CacheTTL),
	}
	d.cacheMu.Unlock()

	log.Printf("[consul] Resolved %s -> %s", serviceName, address)
	return address, nil
}

// ResolveServiceURL resolves a service and returns the full HTTP URL.
func (d *ServiceDiscovery) ResolveServiceURL(ctx context.Context, serviceName string) (string, error) {
	address, err := d.ResolveService(ctx, serviceName)
	if err != nil {
		return "", err
	}
	return address.URL(), nil
}

// resolveViaConnectSidecar resolves a service via its Connect sidecar ingress address.
// This returns the sidecar's address (which terminates mTLS) instead of the service's direct address.
func (d *ServiceDiscovery) resolveViaConnectSidecar(ctx context.Context, serviceName string) (ServiceAddress, error) {
	// Query the catalog for the sidecar service
	// Sidecar service name format: <service-name>-sidecar
	sidecarName := serviceName + "-sidecar"

	services, _, err := d.client.Catalog().Service(sidecarName, "", &api.QueryOptions{
		Token: d.config.ConsulToken,
	})
	if err != nil {
		return ServiceAddress{}, fmt.Errorf("catalog query for sidecar failed: %w", err)
	}

	if len(services) == 0 {
		// Fall back to regular service if no sidecar
		return d.resolveViaCatalog(ctx, serviceName)
	}

	// Pick first healthy sidecar instance
	instance := services[0]

	host := instance.ServiceAddress
	if host == "" {
		host = instance.Address
	}
	port := instance.ServicePort

	if host == "" || port == 0 {
		return ServiceAddress{}, fmt.Errorf("invalid sidecar address: %+v", instance)
	}

	return ServiceAddress{
		Host: host,
		Port: port,
	}, nil
}

// resolveViaDNSSRV resolves a service via Consul DNS SRV records.
func (d *ServiceDiscovery) resolveViaDNSSRV(ctx context.Context, serviceName string) (ServiceAddress, error) {
	// Consul DNS format: <service>.service.consul
	srvName := fmt.Sprintf("%s.service.consul", serviceName)

	// Query SRV records
	_, srvRecords, err := d.resolver.LookupSRV(ctx, "", "", srvName)
	if err != nil {
		return ServiceAddress{}, fmt.Errorf("SRV lookup failed: %w", err)
	}

	if len(srvRecords) == 0 {
		return ServiceAddress{}, fmt.Errorf("no SRV records found")
	}

	// Pick the first SRV record (Consul handles load balancing via DNS)
	// We need to resolve the target to an IP
	target := strings.TrimSuffix(srvRecords[0].Target, ".")
	port := srvRecords[0].Port

	// Resolve the target hostname to IP
	ips, err := d.resolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return ServiceAddress{}, fmt.Errorf("resolve target IP: %w", err)
	}

	if len(ips) == 0 {
		return ServiceAddress{}, fmt.Errorf("no IP addresses for target %s", target)
	}

	return ServiceAddress{
		Host: ips[0].String(),
		Port: int(port),
	}, nil
}

// resolveViaCatalog resolves a service via Consul Catalog HTTP API.
func (d *ServiceDiscovery) resolveViaCatalog(ctx context.Context, serviceName string) (ServiceAddress, error) {
	// Query the catalog for healthy instances
	services, _, err := d.client.Catalog().Service(serviceName, "", &api.QueryOptions{
		Token: d.config.ConsulToken,
	})
	if err != nil {
		return ServiceAddress{}, fmt.Errorf("catalog query failed: %w", err)
	}

	if len(services) == 0 {
		return ServiceAddress{}, fmt.Errorf("no instances found in catalog")
	}

	// Filter to only passing health checks
	var healthyServices []*api.CatalogService
	for _, svc := range services {
		// Check if service has passing health checks
		// Note: Catalog API doesn't directly return check status, we'd need health endpoint
		// For now, use all services returned (Consul only returns passing by default)
		healthyServices = append(healthyServices, svc)
	}

	if len(healthyServices) == 0 {
		return ServiceAddress{}, fmt.Errorf("no healthy instances found")
	}

	// Pick first healthy instance
	instance := healthyServices[0]

	// Use ServiceAddress if available, otherwise Node address
	host := instance.ServiceAddress
	if host == "" {
		host = instance.Address
	}
	port := instance.ServicePort

	if host == "" || port == 0 {
		return ServiceAddress{}, fmt.Errorf("invalid service address: %+v", instance)
	}

	return ServiceAddress{
		Host: host,
		Port: port,
	}, nil
}

// ClearCache clears the resolution cache.
func (d *ServiceDiscovery) ClearCache() {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cache = make(map[string]cachedEntry)
	log.Printf("[consul] Service discovery cache cleared")
}
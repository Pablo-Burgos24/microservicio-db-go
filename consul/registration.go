package consul

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// Registrar handles service registration and health checks with Consul.
type Registrar struct {
	client       *api.Client
	serviceID    string
	serviceName  string
	servicePort  int
	healthPath   string
	deregister   chan struct{}
	deregistered chan struct{} // closed when deregistration is complete
	config       Config        // stored config for access to tags/meta
}

// Config holds the configuration for Consul registration.
type Config struct {
	// ServiceName is the name to register in Consul (e.g., "microservicio-db").
	ServiceName string
	// ServicePort is the port the service listens on.
	ServicePort int
	// HealthPath is the HTTP path for health checks (e.g., "/health").
	HealthPath string
	// ConsulAddress is the Consul agent address (default: "consul:8500").
	ConsulAddress string
	// ConsulToken is the ACL token for registration (optional).
	ConsulToken string
	// Tags are optional tags for the service (e.g., for Traefik routing).
	Tags []string
	// Meta is optional metadata for the service.
	Meta map[string]string
}

// NewRegistrar creates a new Consul registrar.
func NewRegistrar(cfg Config) (*Registrar, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if cfg.ServicePort == 0 {
		return nil, fmt.Errorf("service port is required")
	}
	if cfg.HealthPath == "" {
		cfg.HealthPath = "/health"
	}
	if cfg.ConsulAddress == "" {
		cfg.ConsulAddress = "consul:8500"
	}

	consulConfig := api.DefaultConfig()
	consulConfig.Address = cfg.ConsulAddress
	if cfg.ConsulToken != "" {
		consulConfig.Token = cfg.ConsulToken
	}

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	// Generate unique service ID using hostname + PID
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	serviceID := fmt.Sprintf("%s-%s-%d", cfg.ServiceName, hostname, pid)

	return &Registrar{
		client:       client,
		serviceID:    serviceID,
		serviceName:  cfg.ServiceName,
		servicePort:  cfg.ServicePort,
		healthPath:   cfg.HealthPath,
		deregister:   make(chan struct{}),
		deregistered: make(chan struct{}),
		config:       cfg,
	}, nil
}

// Register registers the service with Consul and starts the health check.
func (r *Registrar) Register(ctx context.Context) error {
	// Get the local IP address for service address
	addr, err := r.getLocalIP()
	if err != nil {
		log.Printf("warning: could not determine local IP, using empty: %v", err)
		addr = ""
	}

	// Build the service registration
	registration := &api.AgentServiceRegistration{
		ID:      r.serviceID,
		Name:    r.serviceName,
		Port:    r.servicePort,
		Address: addr,
		Check: &api.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d%s", addr, r.servicePort, r.healthPath),
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
		Tags: r.buildTags(),
		Meta: r.buildMeta(),
		// Enable Connect sidecar support
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				// Empty registration with just native/connect config
			},
		},
	}

	// Register the service
	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("register service: %w", err)
	}

	log.Printf("Registered service %s (ID: %s) with Consul at %s:%d%s",
		r.serviceName, r.serviceID, addr, r.servicePort, r.healthPath)

	return nil
}

// Deregister deregisters the service from Consul.
func (r *Registrar) Deregister() error {
	if err := r.client.Agent().ServiceDeregister(r.serviceID); err != nil {
		return fmt.Errorf("deregister service: %w", err)
	}
	log.Printf("Deregistered service %s (ID: %s) from Consul", r.serviceName, r.serviceID)
	return nil
}

// WaitForShutdown waits for the deregister signal and then deregisters.
// Call this in a goroutine, then signal shutdown by closing the deregister channel.
func (r *Registrar) WaitForShutdown() {
	<-r.deregister
	_ = r.Deregister()
	close(r.deregistered)
}

// SignalShutdown signals the registrar to deregister and waits for completion.
func (r *Registrar) SignalShutdown() {
	close(r.deregister)
	<-r.deregistered
}

// getLocalIP returns the non-loopback IP address of the container.
func (r *Registrar) getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no non-loopback IPv4 address found")
}

// buildTags builds the tags for service registration.
func (r *Registrar) buildTags() []string {
	tags := []string{"consul-registered"}
	// Add custom tags from config
	tags = append(tags, r.config.Tags...)
	
	// Add Traefik tags for Consul Catalog provider (Phase 6)
	// These tags are read by Traefik's consulCatalog provider
	tags = append(tags, "traefik.enable=true")
	
	return tags
}

// buildMeta builds the metadata for service registration.
func (r *Registrar) buildMeta() map[string]string {
	meta := map[string]string{
		"registered_at": time.Now().UTC().Format(time.RFC3339),
		"version":       "1.0.0",
	}
	return meta
}

// ParsePort parses a port string (e.g., "8080" or ":8080") to int.
func ParsePort(portStr string) (int, error) {
	portStr = strings.TrimPrefix(portStr, ":")
	return strconv.Atoi(portStr)
}
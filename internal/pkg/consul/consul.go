// Package consul provides an interface to Consul for Zeno's use.
package consul

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

// Register creates a Consul client and registers the service along with a TTL health check.
// If the program stops updating the TTL, Consul will deregister the service after 30 seconds.
func Register(ctx context.Context) error {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "consul.Register",
	})

	// Create a default Consul client.
	client, err := api.NewClient(&api.Config{
		Address: config.Get().ConsulAddress + ":" + config.Get().ConsulPort,
		Token:   config.Get().ConsulACLToken,
	})
	if err != nil {
		return fmt.Errorf("failed to create Consul client: %v", err)
	}

	// Get the hostname via env or via command
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname for Pyroscope: %w", err)
	}

	// Configuration for the service registration.
	serviceID := fmt.Sprintf("zeno-%s-%s-%s", hostname, config.Get().Job, uuid.New().String()[:5])
	serviceName := "zeno"
	servicePort := config.Get().APIPort
	serviceTags := config.Get().ConsulRegisterTags

	// Build the service registration with a TTL check.
	registration := &api.AgentServiceRegistration{
		ID:   serviceID,
		Name: serviceName,
		Port: servicePort,
		Tags: serviceTags,
		Check: &api.AgentServiceCheck{
			// The TTL within which a heartbeat must be sent.
			TTL: "30s",
			// Automatically deregister if the check remains critical for 30s.
			DeregisterCriticalServiceAfter: "30s",
		},
	}

	// Register the service with Consul.
	if err := client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service: %v", err)
	}

	logger.Info("registered service", "serviceID", serviceID, "serviceName", serviceName, "serviceTags", serviceTags)

	// Start a goroutine to periodically update the TTL check.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			// Update the TTL to indicate the service is healthy.
			err := client.Agent().UpdateTTL("service:"+serviceID, "passing", api.HealthPassing)
			if err != nil {
				logger.Error("failed to update TTL", "serviceID", serviceID, "error", err)
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				// De-register the service when the context is canceled.
				if err := client.Agent().ServiceDeregister(serviceID); err != nil {
					logger.Error("failed to deregister service", "serviceID", serviceID, "error", err)
				}
				return
			}
		}
	}()

	return nil
}

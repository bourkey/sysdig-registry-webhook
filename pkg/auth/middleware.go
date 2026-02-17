package auth

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/sysdig/registry-webhook-scanner/pkg/config"
)

// Authenticator handles webhook authentication
type Authenticator struct {
	config *config.Config
	logger *logrus.Logger
}

// NewAuthenticator creates a new Authenticator instance
func NewAuthenticator(cfg *config.Config, logger *logrus.Logger) *Authenticator {
	return &Authenticator{
		config: cfg,
		logger: logger,
	}
}

// Middleware returns an HTTP middleware that authenticates webhook requests
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Determine which registry this webhook is for
		// For now, we'll try to authenticate against all configured registries
		// In production, you might want to use different endpoints per registry
		// or identify the registry from the webhook payload

		authenticated := false
		var lastError error

		for _, registry := range a.config.Registries {
			var err error

			switch registry.Auth.Type {
			case "hmac":
				err = VerifyHMAC(r, registry.Auth.Secret)
			case "bearer":
				err = VerifyBearerToken(r, registry.Auth.Secret)
			case "none":
				// No authentication required
				authenticated = true
				continue
			default:
				a.logger.WithFields(logrus.Fields{
					"registry": registry.Name,
					"auth_type": registry.Auth.Type,
				}).Warn("Unknown authentication type")
				continue
			}

			if err == nil {
				authenticated = true
				a.logger.WithFields(logrus.Fields{
					"registry": registry.Name,
					"auth_type": registry.Auth.Type,
				}).Debug("Webhook authenticated")
				break
			}

			lastError = err
		}

		if !authenticated {
			a.logger.WithFields(logrus.Fields{
				"remote_addr": r.RemoteAddr,
				"error": lastError,
			}).Warn("Authentication failed")

			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"authentication failed"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthenticateRegistry authenticates a request for a specific registry
func (a *Authenticator) AuthenticateRegistry(r *http.Request, registryName string) error {
	// Find registry config
	var registryConfig *config.RegistryConfig
	for i := range a.config.Registries {
		if a.config.Registries[i].Name == registryName {
			registryConfig = &a.config.Registries[i]
			break
		}
	}

	if registryConfig == nil {
		return fmt.Errorf("registry not found: %s", registryName)
	}

	// Verify based on auth type
	switch registryConfig.Auth.Type {
	case "hmac":
		return VerifyHMAC(r, registryConfig.Auth.Secret)
	case "bearer":
		return VerifyBearerToken(r, registryConfig.Auth.Secret)
	case "none":
		return nil
	default:
		return fmt.Errorf("unsupported auth type: %s", registryConfig.Auth.Type)
	}
}

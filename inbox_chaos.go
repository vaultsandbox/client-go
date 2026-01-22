package vaultsandbox

import "context"

// GetChaosConfig retrieves the current chaos configuration for this inbox.
// Returns the chaos settings including latency, connection drop, random error,
// greylist, and blackhole configurations.
func (i *Inbox) GetChaosConfig(ctx context.Context) (*ChaosConfig, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := i.client.apiClient.GetInboxChaosConfig(ctx, i.emailAddress)
	if err != nil {
		return nil, err
	}

	return chaosConfigFromDTO(dto), nil
}

// SetChaosConfig creates or updates the chaos configuration for this inbox.
// This allows you to inject various failure scenarios for testing email delivery resilience.
func (i *Inbox) SetChaosConfig(ctx context.Context, config *ChaosConfig) (*ChaosConfig, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	req := chaosConfigToRequest(config)
	dto, err := i.client.apiClient.SetInboxChaosConfig(ctx, i.emailAddress, req)
	if err != nil {
		return nil, err
	}

	return chaosConfigFromDTO(dto), nil
}

// DisableChaos disables all chaos for this inbox.
// This is equivalent to calling SetChaosConfig with Enabled: false.
func (i *Inbox) DisableChaos(ctx context.Context) error {
	if err := i.client.checkClosed(); err != nil {
		return err
	}

	return i.client.apiClient.DisableInboxChaos(ctx, i.emailAddress)
}

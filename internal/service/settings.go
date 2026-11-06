package service

import (
	"context"

	"z-mihomo-health-check/internal/config"
	"z-mihomo-health-check/internal/domain"
	"z-mihomo-health-check/internal/gateway"
	"z-mihomo-health-check/internal/repository"
)

// SaveConfig activates configuration only after durable persistence succeeds.
func (c *Controller) SaveConfig(filename string, cfg config.Config) error {
	if err := repository.SaveConfig(filename, cfg); err != nil {
		return err
	}
	c.Configure(cfg)
	return nil
}
func (c *Controller) Groups(ctx context.Context, cfg config.Config) ([]domain.ProxyGroup, error) {
	return gateway.NewMihomo(cfg).Groups(ctx)
}

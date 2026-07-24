package contentmoderation

import (
	"context"
	"strings"
)

// Chain runs every enabled moderation provider in order. A violation from
// any provider blocks the content; a provider error fails closed. Disabled
// providers are ignored so deployments can enable Baidu independently.
type Chain struct {
	services      []Service
	imageServices []ImageService
}

func NewChain(services ...Service) *Chain {
	active := make([]Service, 0, len(services))
	imageActive := make([]ImageService, 0, len(services))
	for _, service := range services {
		if service == nil {
			continue
		}
		if service.Enabled() {
			active = append(active, service)
		}
		if imageService, ok := service.(ImageService); ok && imageService.ImageEnabled() {
			imageActive = append(imageActive, imageService)
		}
	}
	return &Chain{services: active, imageServices: imageActive}
}

func (c *Chain) Enabled() bool { return c != nil && len(c.services) > 0 }

func (c *Chain) ImageEnabled() bool { return c != nil && len(c.imageServices) > 0 }

func (c *Chain) Review(ctx context.Context, contentType, content string) (Result, error) {
	if c == nil || len(c.services) == 0 {
		return Result{Model: "disabled"}, nil
	}
	models := make([]string, 0, len(c.services))
	for _, service := range c.services {
		result, err := service.Review(ctx, contentType, content)
		if result.Model != "" && result.Model != "disabled" && result.Model != "baidu_disabled" {
			models = appendUniqueModel(models, result.Model)
		}
		if err != nil {
			result.Model = strings.Join(models, "+")
			if result.Model == "" {
				result.Model = "moderation_chain"
			}
			return result, err
		}
		if result.Violation {
			result.Model = strings.Join(models, "+")
			return result, nil
		}
	}
	return Result{Model: strings.Join(models, "+")}, nil
}

func (c *Chain) ReviewImage(ctx context.Context, contentType string, image []byte) (Result, error) {
	if c == nil || len(c.imageServices) == 0 {
		return Result{Model: "image_disabled"}, nil
	}
	models := make([]string, 0, len(c.imageServices))
	for _, service := range c.imageServices {
		result, err := service.ReviewImage(ctx, contentType, image)
		if result.Model != "" && result.Model != "image_disabled" && result.Model != baiduImageDisabledModel {
			models = appendUniqueModel(models, result.Model)
		}
		if err != nil {
			result.Model = strings.Join(models, "+")
			if result.Model == "" {
				result.Model = "image_moderation_chain"
			}
			return result, err
		}
		if result.Violation {
			result.Model = strings.Join(models, "+")
			return result, nil
		}
	}
	return Result{Model: strings.Join(models, "+")}, nil
}

func appendUniqueModel(models []string, model string) []string {
	for _, existing := range models {
		if existing == model {
			return models
		}
	}
	return append(models, model)
}

// FromEnvService builds the complete configured moderation chain.
func FromEnvService() (Service, error) {
	grok, err := FromEnv()
	if err != nil {
		return nil, err
	}
	baidu, err := BaiduFromEnv()
	if err != nil {
		return nil, err
	}
	return NewChain(grok, baidu), nil
}

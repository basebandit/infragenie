package kong

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/basebandit/infragenie/pkg/models"
)

type AdminClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewAdminClient(baseURL, token string) *AdminClient {
	return &AdminClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *AdminClient) ApplyConfiguration(ctx context.Context, config *models.KongResource) error {
	switch config.Type {
	case "service":
		return c.createOrUpdateService(ctx, config)
	case "route":
		return c.createOrUpdateRoute(ctx, config)
	case "plugin":
		return c.createOrUpdateRoute(ctx, config)
	default:
		return fmt.Errorf("unsupported kong resource type: %s", config.Type)
	}
}

func (c *AdminClient) createOrUpdateService(ctx context.Context, config *models.KongResource) error {
	endpoint := fmt.Sprintf("%s/services", c.baseURL)

	serviceConfig := map[string]interface{}{
		"name":     config.Name,
		"url":      config.Config["url"],
		"protocol": config.Config["protocol"],
		"host":     config.Config["host"],
		"port":     config.Config["port"],
		"path":     config.Config["path"],
		"tags":     config.Tags,
	}

	return c.makeRequest(ctx, "POST", endpoint, serviceConfig)
}

func (c *AdminClient) createOrUpdateRoute(ctx context.Context, config *models.KongResource) error {
	endpoint := fmt.Sprintf("%s/routes", c.baseURL)

	routeConfig := map[string]interface{}{
		"name":      config.Name,
		"protocols": config.Config["protocols"],
		"methods":   config.Config["methods"],
		"hosts":     config.Config["hosts"],
		"paths":     config.Config["paths"],
		"service":   config.Config["service"],
		"tags":      config.Tags,
	}

	return c.makeRequest(ctx, "POST", endpoint, routeConfig)
}

func (c *AdminClient) createOrUpdatePlugin(ctx context.Context, config *models.KongResource) error {
	endpoint := fmt.Sprintf("%s/plugins", c.baseURL)

	pluginConfig := map[string]interface{}{
		"name":    config.Name,
		"config":  config.Config["config"],
		"service": config.Config["service"],
		"route":   config.Config["route"],
		"tags":    config.Tags,
	}

	return c.makeRequest(ctx, "POST", endpoint, pluginConfig)
}

func (c *AdminClient) makeRequest(ctx context.Context, method, url string, payload interface{}) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Kong-Admin-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Kong API error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *AdminClient) GetServices(ctx context.Context) ([]models.KongResource, error) {
	endpoint := fmt.Sprintf("%s/services", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Kong-Admin-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Data []map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	var services []models.KongResource
	for _, service := range response.Data {
		kongResource := models.KongResource{
			Type:   "service",
			Name:   service["name"].(string),
			Config: service,
		}
		if tags, ok := service["tags"].([]interface{}); ok {
			for _, tag := range tags {
				kongResource.Tags = append(kongResource.Tags, tag.(string))
			}
		}
		services = append(services, kongResource)
	}

	return services, nil
}

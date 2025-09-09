package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/basebandit/infragenie/internal/kubernetes"
	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/basebandit/infragenie/pkg/utils"
	"github.com/google/uuid"
)

type MonitoringAgent struct {
	BaseAgent
	llmClient  *llm.Client
	kubeClient *kubernetes.Client
	logger     *utils.Logger
}

func NewMonitoringAgent(llmClient *llm.Client, kubeClient *kubernetes.Client, logger *utils.Logger) *MonitoringAgent {
	return &MonitoringAgent{
		BaseAgent: BaseAgent{
			name:      "MonitoringAgent",
			agentType: models.AgentTypeMonitoring,
			capabilities: []string{
				"prometheus-configuration",
				"grafana-dashboards",
				"alert-rule-creation",
				"log-aggregation-setup",
				"distributed-tracing",
				"sli-slo-definition",
				"custom-metrics",
			},
		},
		llmClient:  llmClient,
		kubeClient: kubeClient,
		logger:     logger,
	}
}

func (a *MonitoringAgent) ValidateTask(task *models.Task) error {
	if task.Type != models.TaskTypeMonitoring {
		return fmt.Errorf("invalid task type: expected %s, got %s", models.TaskTypeMonitoring, task.Type)
	}
	return nil
}

func (a *MonitoringAgent) Execute(ctx context.Context, task *models.Task) (*models.Result, error) {
	startTime := time.Now()
	a.logger.Info(fmt.Sprintf("Executing monitoring task: %s", task.ID))

	// Parse monitoring request
	services := task.Context["services"]
	prompt := fmt.Sprintf(llm.MonitoringPrompt, task.Input, services)

	// Generate monitoring configuration
	response, err := a.llmClient.GenerateResponse(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate monitoring config: %w", err)
	}

	// Parse monitoring response
	artifacts, err := a.parseMonitoringResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse monitoring response: %w", err)
	}

	// Deploy monitoring stack
	deploymentResult, err := a.deployMonitoring(ctx, artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy monitoring: %w", err)
	}

	result := &models.Result{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    models.TaskStatusCompleted,
		Output:    deploymentResult,
		Artifacts: *artifacts,
		Metadata: map[string]interface{}{
			"execution_time_ms":  time.Since(startTime).Milliseconds(),
			"monitoring_configs": len(artifacts.MonitoringConfigs),
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExecutionTime: time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

func (a *MonitoringAgent) parseMonitoringResponse(response string) (*models.GeneratedArtifacts, error) {
	var parsed struct {
		PrometheusConfigs []models.MonitoringConfig `json:"prometheus_configs"`
		GrafanaDashboards []models.MonitoringConfig `json:"grafana_dashboards"`
		AlertRules        []models.MonitoringConfig `json:"alert_rules"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	artifacts := &models.GeneratedArtifacts{
		MonitoringConfigs: append(parsed.PrometheusConfigs,
			append(parsed.GrafanaDashboards, parsed.AlertRules...)...),
	}

	return artifacts, nil
}

func (a *MonitoringAgent) deployMonitoring(ctx context.Context, artifacts *models.GeneratedArtifacts) (string, error) {
	var results []string

	for _, config := range artifacts.MonitoringConfigs {
		// Convert monitoring config to Kubernetes resources
		manifest := a.convertToKubernetesManifest(config)
		err := a.kubeClient.ApplyResource(ctx, &manifest)
		if err != nil {
			return "", fmt.Errorf("failed to deploy monitoring config %s: %w", config.Name, err)
		}
		results = append(results, fmt.Sprintf("Deployed %s: %s", config.Type, config.Name))
	}

	return fmt.Sprintf("Monitoring stack deployed:\n%s", strings.Join(results, "\n")), nil
}

func (a *MonitoringAgent) convertToKubernetesManifest(config models.MonitoringConfig) models.KubernetesResource {
	return models.KubernetesResource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: map[string]interface{}{
			"name":      config.Name,
			"namespace": "monitoring",
			"labels":    config.Labels,
		},
		Spec: map[string]interface{}{
			"data": config.Config,
		},
	}
}

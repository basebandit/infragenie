package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/basebandit/infragenie/internal/kong"
	"github.com/basebandit/infragenie/internal/kubernetes"
	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/basebandit/infragenie/pkg/utils"
	"github.com/google/uuid"
)

type InfrastructureAgent struct {
	BaseAgent
	llmClient  *llm.Client
	kubeClient *kubernetes.Client
	kongClient *kong.AdminClient
	logger     *utils.Logger
}

func NewInfrastructureAgent(llmClient *llm.Client, kubeClient *kubernetes.Client, kongClient *kong.AdminClient, logger *utils.Logger) *InfrastructureAgent {
	return &InfrastructureAgent{
		BaseAgent: BaseAgent{
			name:      "InfrastructureAgent",
			agentType: models.AgentTypeInfrastructure,
			capabilities: []string{
				"kubernetes-manifest-generation",
				"kong-gateway-configuration",
				"helm-chart-creation",
				"terraform-module-generation",
				"auto-scaling-configuration",
				"load-balancer-setup",
			},
		},
		llmClient:  llmClient,
		kubeClient: kubeClient,
		kongClient: kongClient,
		logger:     logger,
	}
}

func (a *InfrastructureAgent) ValidateTask(task *models.Task) error {
	if task.Type != models.TaskTypeInfrastructure {
		return fmt.Errorf("invalid task type: expected %s, got %s", models.TaskTypeInfrastructure, task.Type)
	}

	if task.Input == "" {
		return fmt.Errorf("task input cannot be empty")
	}

	return nil
}

func (a *InfrastructureAgent) Execute(ctx context.Context, task *models.Task) (*models.Result, error) {
	startTime := time.Now()
	a.logger.Info(fmt.Sprintf("Executing infrastructure task: %s", task.ID))

	// Validate task
	if err := a.ValidateTask(task); err != nil {
		return nil, err
	}

	// Parse infrastructure request
	var request models.InfrastructureRequest
	if err := json.Unmarshal([]byte(task.Input), &request); err != nil {
		return nil, fmt.Errorf("failed to parse infrastructure request: %w", err)
	}

	// Generate infrastructure using LLM
	prompt := a.buildPrompt(request)
	response, err := a.llmClient.GenerateResponse(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate infrastructure: %w", err)
	}

	// Parse LLM response
	artifacts, err := a.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Deploy infrastructure
	deploymentResult, err := a.deployInfrastructure(ctx, artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy infrastructure: %w", err)
	}

	// Create result
	result := &models.Result{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    models.TaskStatusCompleted,
		Output:    deploymentResult,
		Artifacts: *artifacts,
		Metadata: map[string]interface{}{
			"execution_time_ms": time.Since(startTime).Milliseconds(),
			"resources_created": len(artifacts.KubernetesManifests),
			"kong_configs":      len(artifacts.KongConfigurations),
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExecutionTime: time.Since(startTime).Milliseconds(),
	}

	a.logger.Info(fmt.Sprintf("Infrastructure task completed: %s", task.ID))
	return result, nil
}

func (a *InfrastructureAgent) buildPrompt(request models.InfrastructureRequest) string {
	return fmt.Sprintf(llm.InfrastructurePrompt,
		request.NaturalLanguage,
		request.Context,
		request.Environment,
		request.Requirements.ServiceType,
		request.Requirements.Runtime,
		request.Requirements.Scaling.MinReplicas,
		request.Requirements.Scaling.MaxReplicas,
		request.Requirements.Networking.ExposeExternal,
		request.Requirements.Security.Authentication,
		request.Requirements.Monitoring,
	)
}

func (a *InfrastructureAgent) parseResponse(response string) (*models.GeneratedArtifacts, error) {
	var parsed struct {
		Analysis        string                    `json:"analysis"`
		Artifacts       models.GeneratedArtifacts `json:"artifacts"`
		Recommendations []string                  `json:"recommendations"`
		SecurityNotes   []string                  `json:"security_notes"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	return &parsed.Artifacts, nil
}

func (a *InfrastructureAgent) deployInfrastructure(ctx context.Context, artifacts *models.GeneratedArtifacts) (string, error) {
	var results []string

	// Deploy Kubernetes resources
	for _, manifest := range artifacts.KubernetesManifests {
		err := a.kubeClient.ApplyResource(ctx, &manifest)
		if err != nil {
			return "", fmt.Errorf("failed to deploy K8s resource %s/%s: %w", manifest.Kind, manifest.Metadata["name"], err)
		}
		results = append(results, fmt.Sprintf("Created %s: %s", manifest.Kind, manifest.Metadata["name"]))
	}

	// Configure Kong Gateway
	for _, kongConfig := range artifacts.KongConfigurations {
		err := a.kongClient.ApplyConfiguration(ctx, &kongConfig)
		if err != nil {
			return "", fmt.Errorf("failed to configure Kong %s: %w", kongConfig.Type, err)
		}
		results = append(results, fmt.Sprintf("Configured Kong %s: %s", kongConfig.Type, kongConfig.Name))
	}

	return fmt.Sprintf("Infrastructure deployment completed:\n%s", strings.Join(results, "\n")), nil
}

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

type SecurityAgent struct {
	BaseAgent
	llmClient  *llm.Client
	kubeClient *kubernetes.Client
	logger     *utils.Logger
}

func NewSecurityAgent(llmClient *llm.Client, kubeClient *kubernetes.Client, logger *utils.Logger) *SecurityAgent {
	return &SecurityAgent{
		BaseAgent: BaseAgent{
			name:      "SecurityAgent",
			agentType: models.AgentTypeSecurity,
			capabilities: []string{
				"rbac-policy-generation",
				"network-policy-creation",
				"pod-security-standards",
				"secrets-management",
				"tls-configuration",
				"security-scanning",
				"compliance-checking",
			},
		},
		llmClient:  llmClient,
		kubeClient: kubeClient,
		logger:     logger,
	}
}

func (a *SecurityAgent) ValidateTask(task *models.Task) error {
	if task.Type != models.TaskTypeSecurity {
		return fmt.Errorf("invalid task type: expected %s, got %s", models.TaskTypeSecurity, task.Type)
	}
	return nil
}

func (a *SecurityAgent) Execute(ctx context.Context, task *models.Task) (*models.Result, error) {
	startTime := time.Now()
	a.logger.Info(fmt.Sprintf("Executing security task: %s", task.ID))

	// Build security prompt
	prompt := fmt.Sprintf(llm.SecurityPrompt, task.Input, task.Context["security_level"])

	// Generate security configuration
	response, err := a.llmClient.GenerateResponse(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate security configuration: %w", err)
	}

	// Parse and apply security policies
	artifacts, err := a.parseSecurityResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse security response: %w", err)
	}

	// Apply security configurations
	deploymentResult, err := a.deploySecurityPolicies(ctx, artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy security policies: %w", err)
	}

	result := &models.Result{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    models.TaskStatusCompleted,
		Output:    deploymentResult,
		Artifacts: *artifacts,
		Metadata: map[string]interface{}{
			"execution_time_ms": time.Since(startTime).Milliseconds(),
			"policies_created":  len(artifacts.KubernetesManifests),
		},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExecutionTime: time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

func (a *SecurityAgent) parseSecurityResponse(response string) (*models.GeneratedArtifacts, error) {
	// Similar to infrastructure agent but focused on security artifacts
	var parsed struct {
		SecurityPolicies []models.KubernetesResource `json:"security_policies"`
		NetworkPolicies  []models.KubernetesResource `json:"network_policies"`
		RBACPolicies     []models.KubernetesResource `json:"rbac_policies"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}

	artifacts := &models.GeneratedArtifacts{
		KubernetesManifests: append(parsed.SecurityPolicies,
			append(parsed.NetworkPolicies, parsed.RBACPolicies...)...),
	}

	return artifacts, nil
}

func (a *SecurityAgent) deploySecurityPolicies(ctx context.Context, artifacts *models.GeneratedArtifacts) (string, error) {
	var results []string

	for _, policy := range artifacts.KubernetesManifests {
		err := a.kubeClient.ApplyResource(ctx, &policy)
		if err != nil {
			return "", fmt.Errorf("failed to apply security policy %s: %w", policy.Metadata["name"], err)
		}
		results = append(results, fmt.Sprintf("Applied %s: %s", policy.Kind, policy.Metadata["name"]))
	}

	return fmt.Sprintf("Security policies deployed:\n%s", strings.Join(results, "\n")), nil
}

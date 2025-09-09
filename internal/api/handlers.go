package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/internal/orchestrator"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/basebandit/infragenie/pkg/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	orchestrator *orchestrator.Manager
	llmClient    *llm.Client
	logger       *utils.Logger
}

func NewHandler(orchestrator *orchestrator.Manager, llmClient *llm.Client, logger *utils.Logger) *Handler {
	return &Handler{
		orchestrator: orchestrator,
		llmClient:    llmClient,
		logger:       logger,
	}
}

// CreateInfrastructure handles infrastructure generation requests
func (h *Handler) CreateInfrastructure(c *gin.Context) {
	var request models.InfrastructureRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if request.NaturalLanguage == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "natural_language field is required",
		})
		return
	}

	// Set defaults
	if request.Environment == "" {
		request.Environment = "development"
	}

	// Check if any LLM providers are available
	ctx := context.Background()
	availableProviders := h.llmClient.GetAvailableProviders(ctx)
	if len(availableProviders) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":               "No LLM providers are currently available",
			"details":             "Please check your provider configurations and try again",
			"available_providers": []string{},
		})
		return
	}

	// Submit task
	task, err := h.orchestrator.SubmitTask(&request)
	if err != nil {
		h.logger.Error("Failed to submit task: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to submit infrastructure request",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":             task.ID,
		"status":              task.Status,
		"message":             "Infrastructure generation task submitted successfully",
		"available_providers": availableProviders,
		"primary_provider":    h.llmClient.GetPrimaryProvider(),
		"estimated_time":      "30-60 seconds",
	})
}

// GetTaskStatus returns the status of a specific task
func (h *Handler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Task ID is required",
		})
		return
	}

	task, result, err := h.orchestrator.GetTaskStatus(taskID)
	if err != nil {
		h.logger.Error("Failed to get task status: " + err.Error())
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Task not found",
			"details": err.Error(),
		})
		return
	}

	response := gin.H{
		"task_id":    task.ID,
		"status":     task.Status,
		"type":       task.Type,
		"agent_type": task.AgentType,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
		"priority":   task.Priority,
	}

	if task.Status == models.TaskStatusFailed {
		response["error"] = task.ErrorMessage
		response["retry_count"] = task.RetryCount
		response["max_retries"] = task.MaxRetries
	}

	if task.CompletedAt != nil {
		response["completed_at"] = task.CompletedAt
		response["total_duration"] = task.CompletedAt.Sub(task.CreatedAt).String()
	}

	if result != nil {
		response["result"] = gin.H{
			"execution_time_ms": result.ExecutionTime,
			"output":            result.Output,
			"artifacts_count":   len(result.Artifacts.KubernetesManifests),
			"kong_configs":      len(result.Artifacts.KongConfigurations),
			"metadata":          result.Metadata,
		}

		// Include artifacts only if specifically requested
		if c.Query("include_artifacts") == "true" {
			response["artifacts"] = result.Artifacts
		}
	}

	c.JSON(http.StatusOK, response)
}

// ListTasks returns a paginated list of tasks with optional filtering
func (h *Handler) ListTasks(c *gin.Context) {
	// Parse query parameters
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	status := c.Query("status")
	taskType := c.Query("type")
	agentType := c.Query("agent_type")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	// Build filter criteria
	filter := models.TaskFilter{
		Status:    models.TaskStatus(status),
		Type:      models.TaskType(taskType),
		AgentType: models.AgentType(agentType),
		Page:      page,
		Limit:     limit,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	// Get tasks from orchestrator
	tasks, total, err := h.orchestrator.ListTasks(filter)
	if err != nil {
		h.logger.Error("Failed to list tasks: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve tasks",
			"details": err.Error(),
		})
		return
	}

	// Transform tasks for API response
	taskList := make([]gin.H, len(tasks))
	for i, task := range tasks {
		taskItem := gin.H{
			"id":         task.ID,
			"status":     task.Status,
			"type":       task.Type,
			"agent_type": task.AgentType,
			"created_at": task.CreatedAt,
			"updated_at": task.UpdatedAt,
			"priority":   task.Priority,
		}

		if task.CompletedAt != nil {
			taskItem["completed_at"] = task.CompletedAt
			taskItem["duration"] = task.CompletedAt.Sub(task.CreatedAt).String()
		}

		if task.Status == models.TaskStatusFailed {
			taskItem["error"] = task.ErrorMessage
			taskItem["retry_count"] = task.RetryCount
		}

		taskList[i] = taskItem
	}

	// Calculate pagination info
	totalPages := (int(total) + limit - 1) / limit
	hasNext := page < totalPages
	hasPrev := page > 1

	c.JSON(http.StatusOK, gin.H{
		"tasks": taskList,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    hasNext,
			"has_prev":    hasPrev,
		},
		"filters": gin.H{
			"status":     status,
			"type":       taskType,
			"agent_type": agentType,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
		"stats": h.getTaskStats(),
	})
}

// GetHealthCheck returns system health status
func (h *Handler) GetHealthCheck(c *gin.Context) {
	ctx := context.Background()

	// Check LLM provider status
	providerStatus := h.llmClient.GetProviderStatus(ctx)
	availableProviders := h.llmClient.GetAvailableProviders(ctx)

	// Check orchestrator health
	orchestratorStats := h.orchestrator.GetStats()

	// Determine overall health
	overallHealth := "healthy"
	if len(availableProviders) == 0 {
		overallHealth = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    overallHealth,
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"service":   "InfraGenie API",
		"uptime":    time.Since(orchestratorStats.StartTime).String(),
		"providers": gin.H{
			"available": availableProviders,
			"status":    providerStatus,
			"total":     len(providerStatus),
		},
		"orchestrator": gin.H{
			"active_tasks":    orchestratorStats.ActiveTasks,
			"completed_tasks": orchestratorStats.CompletedTasks,
			"failed_tasks":    orchestratorStats.FailedTasks,
			"queue_size":      orchestratorStats.QueueSize,
		},
	})
}

// GetMetrics returns system metrics and statistics
func (h *Handler) GetMetrics(c *gin.Context) {
	ctx := context.Background()

	// Get orchestrator metrics
	orchestratorStats := h.orchestrator.GetStats()

	// Get LLM provider metrics
	providerMetrics := h.llmClient.GetProviderMetrics(ctx)

	// Calculate success rates and averages
	totalTasks := orchestratorStats.CompletedTasks + orchestratorStats.FailedTasks
	successRate := float64(0)
	if totalTasks > 0 {
		successRate = float64(orchestratorStats.CompletedTasks) / float64(totalTasks) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"system_metrics": gin.H{
			"total_requests":    orchestratorStats.TotalRequests,
			"completed_tasks":   orchestratorStats.CompletedTasks,
			"failed_tasks":      orchestratorStats.FailedTasks,
			"active_tasks":      orchestratorStats.ActiveTasks,
			"success_rate":      successRate,
			"average_exec_time": orchestratorStats.AverageExecutionTime,
			"uptime":            time.Since(orchestratorStats.StartTime).String(),
		},
		"provider_metrics": providerMetrics,
		"task_stats":       h.getTaskStats(),
		"timestamp":        time.Now().Unix(),
	})
}

// GetLLMProviders returns information about configured LLM providers
func (h *Handler) GetLLMProviders(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providers := h.llmClient.GetAvailableProviders(ctx)
	status := h.llmClient.GetProviderStatus(ctx)
	metrics := h.llmClient.GetProviderMetrics(ctx)

	// Build detailed provider information
	providerInfo := make(map[string]gin.H)
	for provider := range status {
		info := gin.H{
			"available": status[provider],
			"type":      getProviderDisplayName(string(provider)),
			"primary":   provider == h.llmClient.GetPrimaryProvider(),
		}

		if providerMetric, exists := metrics[provider]; exists {
			info["metrics"] = providerMetric
		}

		providerInfo[string(provider)] = info
	}

	response := gin.H{
		"available_providers": providers,
		"provider_status":     status,
		"provider_info":       providerInfo,
		"total_providers":     len(status),
		"primary_provider":    h.llmClient.GetPrimaryProvider(),
		"fallback_enabled":    len(providers) > 1,
		"last_updated":        time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// TestLLMProvider tests connectivity to a specific LLM provider
func (h *Handler) TestLLMProvider(c *gin.Context) {
	var request struct {
		Provider string `json:"provider"`
		Prompt   string `json:"prompt"`
		Timeout  int    `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if request.Prompt == "" {
		request.Prompt = "Hello! This is a test prompt for InfraGenie. Please respond with a brief confirmation that you're working correctly."
	}

	timeout := 30 * time.Second
	if request.Timeout > 0 && request.Timeout <= 120 {
		timeout = time.Duration(request.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	response, err := h.llmClient.GenerateResponse(ctx, request.Prompt)
	responseTime := time.Since(startTime)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":            "LLM provider test failed",
			"details":          err.Error(),
			"provider":         request.Provider,
			"response_time_ms": responseTime.Milliseconds(),
			"status":           "failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":         h.llmClient.GetPrimaryProvider(),
		"response":         response,
		"status":           "success",
		"response_time_ms": responseTime.Milliseconds(),
		"prompt_length":    len(request.Prompt),
		"response_length":  len(response),
		"test_timestamp":   time.Now().Unix(),
	})
}

// Helper function to get task statistics
func (h *Handler) getTaskStats() gin.H {
	stats := h.orchestrator.GetStats()

	return gin.H{
		"total":     stats.TotalRequests,
		"completed": stats.CompletedTasks,
		"failed":    stats.FailedTasks,
		"active":    stats.ActiveTasks,
		"queued":    stats.QueueSize,
	}
}

// Helper function to get provider display name
func getProviderDisplayName(provider string) string {
	names := map[string]string{
		"openai":    "OpenAI GPT Models",
		"anthropic": "Anthropic Claude",
		"google":    "Google Gemini",
		"azure":     "Azure OpenAI",
		"aws":       "AWS Bedrock",
		"local":     "Local LLM (Ollama/LocalAI)",
	}
	if name, exists := names[provider]; exists {
		return name
	}
	return strings.Title(provider)
}

// Package models - contains all data models and schema for internal data
package models

import (
	"time"

	"github.com/google/uuid"
)

type (
	TaskType   string
	TaskStatus string
	AgentType  string
)

const (
	TaskTypeInfrastructure TaskType = "infrastructure"
	TaskTypeSecurity       TaskType = "security"
	TaskTypeMonitoring     TaskType = "monitoring"
	TaskTypeDocumentation  TaskType = "documentation"
	TaskTypeOptimization   TaskType = "optimization"

	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"

	AgentTypeInfrastructure AgentType = "infrastructure"
	AgentTypeSecurity       AgentType = "security"
	AgentTypeMonitoring     AgentType = "monitoring"
	AgentTypeDocumentation  AgentType = "documentation"
)

type Task struct {
	ID           uuid.UUID              `json:"id" gorm:"type:uuid:primary_key"`
	Type         TaskType               `json:"type"`
	Status       TaskStatus             `json:"status"`
	AgentType    AgentType              `json:"agent_type"`
	Input        string                 `json:"input"`
	Context      map[string]interface{} `json:"context" gorm:"type:jsonb"`
	Parameters   map[string]interface{} `json:"parameters" gorm:"type:jsonb"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	Priority     int                    `json:"priority"`
}

type TaskStats struct {
	Total     int64 `json:"total" gorm:"column:total"`
	Pending   int64 `json:"pending" gorm:"column:pending"`
	Running   int64 `json:"running" gorm:"column:running"`
	Completed int64 `json:"completed" gorm:"column:completed"`
	Failed    int64 `json:"failed" gorm:"column:failed"`
}

type TaskFilter struct {
	Status    TaskStatus `json:"status,omitempty"`
	Type      TaskType   `json:"type,omitempty"`
	AgentType AgentType  `json:"agent_type,omitempty"`
	Page      int        `json:"page"`
	Limit     int        `json:"limit"`
	SortBy    string     `json:"sort_by"`
	SortOrder string     `json:"sort_order"`
}

type InfrastructureRequest struct {
	NaturalLanguage string            `json:"natural_language" binding:"required"`
	Context         map[string]string `json:"context"`
	Constraints     []string          `json:"constraints"`
	Environment     string            `json:"environment"`
	Requirements    Requirements      `json:"requirements"`
}

type Requirements struct {
	ServiceType  string               `json:"service_type"`
	Runtime      string               `json:"runtime"`
	Dependencies []string             `json:"dependencies"`
	Scaling      Scaling              `json:"scaling"`
	Networking   NetworkRequirements  `json:"networking"`
	Security     SecurityRequirements `json:"security"`
	Monitoring   bool                 `json:"monitoring"`
}

type Scaling struct {
	MinReplicas  int `json:"min_replicas"`
	MaxReplicas  int `json:"max_replicas"`
	TargetCPU    int `json:"target_cpu_percent"`
	TargetMemory int `json:"target_memory_percent"`
}

type NetworkRequirements struct {
	ExposeExternal bool   `json:"expose_external"`
	Ports          []int  `json:"ports"`
	Protocol       string `json:"protocol"`
	LoadBalancer   bool   `json:"load_balancer"`
}

type SecurityRequirements struct {
	Authentication  string `json:"authentication"`
	Authorization   string `json:"authorization"`
	TLS             bool   `json:"tls"`
	NetworkPolicies bool   `json:"network_policies"`
}

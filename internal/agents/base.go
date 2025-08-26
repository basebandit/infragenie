package agents

import (
	"context"
	"time"

	"github.com/basebandit/infragenie/pkg/models"
)

type Agent interface {
	Name() string
	Type() models.AgentType
	Capabilities() []string
	Execute(ctx context.Context, task *models.Task) (*models.Result, error)
	Health() HealthStatus
	ValidateTask(task *models.Task) error
}

type HealthStatus struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type BaseAgent struct {
	name         string
	agentType    models.AgentType
	capabilities []string
}

func (b *BaseAgent) Name() string {
	return b.name
}

func (b *BaseAgent) Type() models.AgentType {
	return b.agentType
}

func (b *BaseAgent) Capabilities() []string {
	return b.capabilities
}

func (b *BaseAgent) Health() HealthStatus {
	return HealthStatus{
		Status:    "healthy",
		Message:   "Agent is operational",
		Timestamp: time.Now().Unix(),
		Metadata:  make(map[string]interface{}),
	}
}

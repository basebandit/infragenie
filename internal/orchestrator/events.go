package orchestrator

import (
	"context"
	"encoding/json"

	"github.com/basebandit/infragenie/internal/agents"
	"github.com/basebandit/infragenie/internal/database"
	"github.com/basebandit/infragenie/pkg/models"
)

type EventPublisher struct {
	redis *database.RedisClient
}

type Event struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func NewEventPublisher(redis *database.RedisClient) *EventPublisher {
	return &EventPublisher{redis: redis}
}

func (e *EventPublisher) PublishTaskCreated(task *models.Task) {
	event := Event{
		Type:      "task.created",
		Timestamp: task.CreatedAt.Unix(),
		Data: map[string]interface{}{
			"task_id":    task.ID.String(),
			"task_type":  task.Type,
			"agent_type": task.AgentType,
		},
	}
	e.publishEvent(event)
}

func (e *EventPublisher) PublishTaskStarted(task *models.Task) {
	event := Event{
		Type:      "task.started",
		Timestamp: task.UpdatedAt.Unix(),
		Data: map[string]interface{}{
			"task_id": task.ID.String(),
		},
	}
	e.publishEvent(event)
}

func (e *EventPublisher) PublishTaskCompleted(task *models.Task, result *models.Result) {
	event := Event{
		Type:      "task.completed",
		Timestamp: task.CompletedAt.Unix(),
		Data: map[string]interface{}{
			"task_id":        task.ID.String(),
			"execution_time": result.ExecutionTime,
			"artifacts":      len(result.Artifacts.KubernetesManifests),
		},
	}
	e.publishEvent(event)
}

func (e *EventPublisher) PublishTaskFailed(task *models.Task) {
	event := Event{
		Type:      "task.failed",
		Timestamp: task.UpdatedAt.Unix(),
		Data: map[string]interface{}{
			"task_id":     task.ID.String(),
			"error":       task.ErrorMessage,
			"retry_count": task.RetryCount,
		},
	}
	e.publishEvent(event)
}

func (e *EventPublisher) PublishAgentHealthAlert(agentType models.AgentType, health agents.HealthStatus) {
	event := Event{
		Type:      "agent.health.alert",
		Timestamp: health.Timestamp,
		Data: map[string]interface{}{
			"agent_type": agentType,
			"status":     health.Status,
			"message":    health.Message,
		},
	}
	e.publishEvent(event)
}

func (e *EventPublisher) publishEvent(event Event) {
	eventJSON, _ := json.Marshal(event)
	// Publish to Redis pub/sub channel
	e.redis.Client.Publish(context.Background(), "infragenie:events", eventJSON)
}

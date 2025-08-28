package orchestrator

import (
	"github.com/basebandit/infragenie/internal/database"
	"github.com/basebandit/infragenie/pkg/models"
)

type TaskQueue struct {
	redis *database.RedisClient
}

func NewTaskQueue(redis *database.RedisClient) *TaskQueue {
	return &TaskQueue{redis: redis}
}

func (q *TaskQueue) Enqueue(task *models.Task) error {
	return q.redis.EnqueueTask(task)
}

func (q *TaskQueue) Dequeue(agentType models.AgentType) (*models.Task, error) {
	return q.redis.DequeueTask(agentType)
}

func (q *TaskQueue) GetQueueStats() (map[models.AgentType]int, error) {
	// Implementation to get queue statistics
	stats := make(map[models.AgentType]int)
	// This would query Redis for queue lengths
	return stats, nil
}

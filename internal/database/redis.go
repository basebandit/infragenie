package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basebandit/infragenie/pkg/models"
	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	Client *redis.Client
}

func NewRedisConnection(redisURL string) *RedisClient {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse Redis URL: %v", err))
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}

	return &RedisClient{Client: client}
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}

func (r *RedisClient) EnqueueTask(task *models.Task) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}

	queueName := fmt.Sprintf("tasks:%s", task.AgentType)
	return r.Client.LPush(context.Background(), queueName, taskJSON).Err()
}

func (r *RedisClient) DequeueTask(agentType models.AgentType) (*models.Task, error) {
	queueName := fmt.Sprintf("tasks:%s", agentType)

	result, err := r.Client.BRPop(context.Background(), 5*time.Second, queueName).Result()
	if err != nil {
		return nil, err
	}

	var task models.Task
	err = json.Unmarshal([]byte(result[1]), &task)
	return &task, err
}

func (r *RedisClient) SetCache(key string, value interface{}, expiration time.Duration) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.Client.Set(context.Background(), key, valueJSON, expiration).Err()
}

func (r *RedisClient) GetCache(key string, dest interface{}) error {
	value, err := r.Client.Get(context.Background(), key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(value), dest)
}

package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/basebandit/infragenie/internal/agents"
	"github.com/basebandit/infragenie/internal/database"
	"github.com/basebandit/infragenie/internal/kong"
	"github.com/basebandit/infragenie/internal/kubernetes"
	"github.com/basebandit/infragenie/internal/llm"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/basebandit/infragenie/pkg/utils"
	"github.com/google/uuid"
)

type Manager struct {
	agents    map[models.AgentType]agents.Agent
	db        *database.PostgresDB
	redis     *database.RedisClient
	taskQueue *TaskQueue
	events    *EventPublisher
	logger    *utils.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewManager(
	db *database.PostgresDB,
	redis *database.RedisClient,
	k8sClient *kubernetes.Client,
	kongClient *kong.AdminClient,
	llmClient *llm.Client,
	logger *utils.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize agents
	agentsMap := make(map[models.AgentType]agents.Agent)
	agentsMap[models.AgentTypeInfrastructure] = agents.NewInfrastructureAgent(llmClient, k8sClient, kongClient, logger)
	agentsMap[models.AgentTypeSecurity] = agents.NewSecurityAgent(llmClient, k8sClient, logger)
	agentsMap[models.AgentTypeMonitoring] = agents.NewMonitoringAgent(llmClient, k8sClient, logger)

	taskQueue := NewTaskQueue(redis)
	events := NewEventPublisher(redis)

	return &Manager{
		agents:    agentsMap,
		db:        db,
		redis:     redis,
		taskQueue: taskQueue,
		events:    events,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting orchestrator manager")

	// Start worker goroutines for each agent type
	for agentType, agent := range m.agents {
		m.wg.Add(1)
		go m.workerLoop(ctx, agentType, agent)
	}

	// Start task scheduler
	m.wg.Add(1)
	go m.taskScheduler(ctx)

	// Start health monitor
	m.wg.Add(1)
	go m.healthMonitor(ctx)

	m.wg.Wait()
	return nil
}

func (m *Manager) Stop() {
	m.logger.Info("Stopping orchestrator manager")
	m.cancel()
	m.wg.Wait()
}

func (m *Manager) SubmitTask(request *models.InfrastructureRequest) (*models.Task, error) {
	task := &models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeInfrastructure,
		Status:     models.TaskStatusPending,
		AgentType:  models.AgentTypeInfrastructure,
		Input:      request.NaturalLanguage,
		Context:    make(map[string]interface{}),
		Parameters: make(map[string]interface{}),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
		MaxRetries: 3,
		Priority:   1,
	}

	// Store context and requirements
	if request.Context != nil {
		for k, v := range request.Context {
			task.Context[k] = v
		}
	}
	task.Parameters["requirements"] = request.Requirements

	// Save task to database
	if err := m.db.CreateTask(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Queue task for processing
	if err := m.taskQueue.Enqueue(task); err != nil {
		return nil, fmt.Errorf("failed to queue task: %w", err)
	}

	m.logger.Info(fmt.Sprintf("Task submitted: %s", task.ID))

	// Publish event
	m.events.PublishTaskCreated(task)

	return task, nil
}

func (m *Manager) GetTaskStatus(taskID string) (*models.Task, *models.Result, error) {
	task, err := m.db.GetTask(taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task: %w", err)
	}

	var result *models.Result
	if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusFailed {
		result, err = m.db.GetResultByTaskID(taskID)
		if err != nil && err.Error() != "record not found" {
			return nil, nil, fmt.Errorf("failed to get result: %w", err)
		}
	}

	return task, result, nil
}

func (m *Manager) workerLoop(ctx context.Context, agentType models.AgentType, agent agents.Agent) {
	defer m.wg.Done()

	m.logger.Info(fmt.Sprintf("Starting worker for agent type: %s", agentType))

	for {
		select {
		case <-ctx.Done():
			m.logger.Info(fmt.Sprintf("Stopping worker for agent type: %s", agentType))
			return
		default:
			// Dequeue task
			task, err := m.taskQueue.Dequeue(agentType)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			m.processTask(ctx, agent, task)
		}
	}
}

func (m *Manager) processTask(ctx context.Context, agent agents.Agent, task *models.Task) {
	m.logger.Info(fmt.Sprintf("Processing task %s with agent %s", task.ID, agent.Name()))

	// Update task status to running
	task.Status = models.TaskStatusRunning
	task.UpdatedAt = time.Now()
	m.db.UpdateTask(task)
	m.events.PublishTaskStarted(task)

	// Execute task
	result, err := agent.Execute(ctx, task)
	if err != nil {
		m.logger.Error(fmt.Sprintf("Task execution failed: %s - %v", task.ID, err))

		// Handle retry logic
		if task.RetryCount < task.MaxRetries {
			task.RetryCount++
			task.Status = models.TaskStatusPending
			task.UpdatedAt = time.Now()
			m.db.UpdateTask(task)
			m.taskQueue.Enqueue(task) // Re-queue for retry
			return
		}

		// Max retries reached, mark as failed
		task.Status = models.TaskStatusFailed
		task.ErrorMessage = err.Error()
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		task.UpdatedAt = completedAt
		m.db.UpdateTask(task)
		m.events.PublishTaskFailed(task)
		return
	}

	// Task completed successfully
	task.Status = models.TaskStatusCompleted
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.UpdatedAt = completedAt
	m.db.UpdateTask(task)

	// Save result
	result.TaskID = task.ID
	m.db.CreateResult(result)

	m.events.PublishTaskCompleted(task, result)
	m.logger.Info(fmt.Sprintf("Task completed successfully: %s", task.ID))
}

func (m *Manager) taskScheduler(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.schedulePendingTasks()
		}
	}
}

func (m *Manager) schedulePendingTasks() {
	tasks, err := m.db.GetPendingTasks()
	if err != nil {
		m.logger.Error(fmt.Sprintf("Failed to get pending tasks: %v", err))
		return
	}

	for _, task := range tasks {
		if err := m.taskQueue.Enqueue(&task); err != nil {
			m.logger.Error(fmt.Sprintf("Failed to enqueue task %s: %v", task.ID, err))
		}
	}
}

func (m *Manager) healthMonitor(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAgentHealth()
		}
	}
}

func (m *Manager) checkAgentHealth() {
	for agentType, agent := range m.agents {
		health := agent.Health()
		m.logger.Info(fmt.Sprintf("Agent %s health: %s", agentType, health.Status))

		if health.Status != "healthy" {
			m.events.PublishAgentHealthAlert(agentType, health)
		}
	}
}

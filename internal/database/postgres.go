package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/basebandit/infragenie/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PostgresDB struct {
	DB *gorm.DB
}

func NewPostgresConnection(databaseURL string) (*PostgresDB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Auto-migrate schemas
	err = db.AutoMigrate(

		&models.Task{},
		&models.Result{},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}

func (p *PostgresDB) Close() error {
	sqlDB, err := p.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

func (p *PostgresDB) CreateTask(task *models.Task) error {
	return p.DB.Create(task).Error
}

func (p *PostgresDB) GetTask(id string) (*models.Task, error) {
	var task models.Task
	err := p.DB.First(&task, "id = ?", id).Error
	return &task, err
}

func (p *PostgresDB) UpdateTask(task *models.Task) error {
	return p.DB.Save(task).Error
}

func (p *PostgresDB) GetPendingTasks() ([]models.Task, error) {
	var tasks []models.Task
	err := p.DB.Where("status = ?", models.TaskStatusPending).
		Order("priority DESC, created_at ASC").
		Find(&tasks).Error

	return tasks, err
}

func (p *PostgresDB) CreateResult(result *models.Result) error {
	return p.DB.Create(result).Error
}

func (p *PostgresDB) GetResultByTaskID(taskID string) (*models.Result, error) {
	var result models.Result
	err := p.DB.Preload("Task").First(&result, "task_id = ?", taskID).Error
	return &result, err
}

func (p *PostgresDB) GetTaskStats() (*models.TaskStats, error) {
	var stats models.TaskStats

	err := p.DB.Raw(`
	 	SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN status = ? THEN 1 END) as pending,
			COUNT(CASE WHEN status = ? THEN 1 END) as running,
			COUNT(CASE WHEN status = ? THEN 1 END) as completed,
			COUNT(CASE WHEN status = ? THEN 1 END) as failed
	  FROM tasks
	`, models.TaskStatusPending, models.TaskStatusRunning, models.TaskStatusCompleted, models.TaskStatusFailed).Scan(&stats).Error

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (p *PostgresDB) GetAverageExecutionTime() (int64, error) {
	var avgTime sql.NullFloat64

	err := p.DB.Raw(`
		SELECT AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) * 1000 as avg_time
		FROM tasks 
		WHERE status = ? AND completed_at IS NOT NULL
	`, models.TaskStatusCompleted).Scan(&avgTime).Error

	if err != nil {
		return 0, err
	}

	if avgTime.Valid {
		return int64(avgTime.Float64), nil
	}
	return 0, nil
}

func (p *PostgresDB) ListTasks(filter models.TaskFilter) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64

	// Build query
	query := p.DB.Model(&models.Task{})

	// Apply filters
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.AgentType != "" {
		query = query.Where("agent_type = ?", filter.AgentType)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	// Apply sorting
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, sortOrder)

	// Apply pagination
	offset := (filter.Page - 1) * filter.Limit
	if err := query.Order(orderClause).Offset(offset).Limit(filter.Limit).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch tasks: %w", err)
	}

	return tasks, total, nil
}

// GetTasksByDateRange returns tasks within a specific date range
func (p *PostgresDB) GetTasksByDateRange(start, end time.Time) ([]models.Task, error) {
	var tasks []models.Task
	err := p.DB.Where("created_at BETWEEN ? AND ?", start, end).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

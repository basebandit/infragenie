package database

import (
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

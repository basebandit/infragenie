package models

import (
	"time"

	"github.com/google/uuid"
)

type Result struct {
	ID            uuid.UUID              `json:"id" gorm:"type:uuid;primary_key"`
	TaskID        uuid.UUID              `json:"task_id" gorm:"type:uuid"`
	Task          Task                   `json:"task" gorm:"foreignKey:TaskID"`
	Status        TaskStatus             `json:"status"`
	Output        string                 `json:"output"`
	Artifacts     GeneratedArtifacts     `json:"artifacts" gorm:"type:jsonb"`
	Metadata      map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	ExecutionTime int64                  `json:"execution_time"` // milliseconds
}

type GeneratedArtifacts struct {
	KubernetesManifests []KubernetesResource `json:"kubernetes_manifests"`
	KongConfigurations  []KongResource       `json:"kong_configurations"`
	HelmChart           HelmChartStructure   `json:"helm_chart"`
	TerraformModules    []TerraformModule    `json:"terraform_modules"`
	Documentation       string               `json:"documentation"`
	MonitoringConfigs   []MonitoringConfig   `json:"monitoring_configs"`
}

type KubernetesResource struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	Status     map[string]interface{} `json:"status,omitempty"`
}

type KongResource struct {
	Type   string                 `json:"type"` // service, route, plugin
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
	Tags   []string               `json:"tags"`
}

type HelmChartStructure struct {
	ChartYaml    map[string]interface{} `json:"chart_yaml"`
	ValuesYaml   map[string]interface{} `json:"values_yaml"`
	Templates    map[string]string      `json:"templates"`
	Dependencies []HelmDependency       `json:"dependencies"`
}

type HelmDependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
}

type TerraformModule struct {
	Name      string                 `json:"name"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
	Variables map[string]interface{} `json:"variables"`
	Outputs   map[string]interface{} `json:"outputs"`
}

type MonitoringConfig struct {
	Type   string                 `json:"type"` // prometheus, grafana, alert
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
	Labels map[string]string      `json:"outputs"`
}

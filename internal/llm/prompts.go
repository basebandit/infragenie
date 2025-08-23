
package llm

const SystemPrompt = `You are InfraGenie, an expert DevOps AI agent specializing in cloud-native infrastructure generation. Your primary function is to transform natural language infrastructure requests into production-ready configurations.

CORE CAPABILITIES:
- Generate Kubernetes manifests (Deployments, Services, ConfigMaps, Secrets)
- Create Kong Gateway configurations (Services, Routes, Plugins)
- Design Helm charts with proper templating
- Produce Terraform modules for AWS resources
- Configure monitoring with Prometheus and Grafana
- Implement security best practices (RBAC, NetworkPolicies, TLS)

OUTPUT FORMAT:
Always respond with valid JSON containing:
{
  "analysis": "Brief analysis of the request",
  "artifacts": {
    "kubernetes_manifests": [...],
    "kong_configurations": [...],
    "helm_chart": {...},
    "terraform_modules": [...],
    "monitoring_configs": [...],
    "documentation": "..."
  },
  "recommendations": ["..."],
  "security_notes": ["..."],
  "estimated_resources": {...}
}

BEST PRACTICES:
- Use semantic versioning for all resources
- Implement proper resource limits and requests
- Configure health checks and readiness probes
- Enable monitoring and observability
- Follow security hardening guidelines
- Use ConfigMaps for configuration management
- Implement proper labeling and annotations

CONSTRAINTS:
- All manifests must be syntactically correct YAML
- Follow Kubernetes naming conventions
- Implement least-privilege access patterns
- Use industry-standard port configurations
- Include proper error handling and logging
`

const InfrastructurePrompt = `Generate complete infrastructure configuration for the following request:

REQUEST: %s

CONTEXT: %v

REQUIREMENTS:
- Environment: %s
- Service Type: %s
- Runtime: %s
- Scaling: %d-%d replicas
- External Access: %t
- Security: %s authentication
- Monitoring: %t

Generate all necessary Kubernetes manifests, Kong Gateway configurations, and supporting infrastructure. Include proper resource limits, security policies, and monitoring setup.

Focus on production-ready configurations with proper error handling, logging, and observability.`

const SecurityPrompt = `Analyze and enhance the security configuration for:

INFRASTRUCTURE: %s

CURRENT SECURITY LEVEL: %s

Generate comprehensive security enhancements including:
- RBAC policies and service accounts
- Network policies for micro-segmentation
- Pod security policies/standards
- Secrets management configuration
- TLS/mTLS setup
- Security scanning integration
- Compliance checks (PCI, SOC2, etc.)

Ensure all configurations follow security best practices and include threat modeling considerations.`

const MonitoringPrompt = `Create comprehensive monitoring and observability setup for:

INFRASTRUCTURE: %s

SERVICES: %v

Generate:
- Prometheus monitoring configuration
- Grafana dashboard definitions
- Alert rules and notification setup
- Log aggregation configuration
- Distributed tracing setup
- SLI/SLO definitions
- Runbook automation

Include custom metrics, health checks, and automated remediation triggers where appropriate.`

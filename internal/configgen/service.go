package configgen

import (
	"context"
	"errors"

	"omo/internal/store"
)

const (
	JobKindServiceConfigApply    = "service_config_apply"
	JobKindServiceConfigRollback = "service_config_rollback"

	StateRender   = "CORE_CONFIG_RENDER"
	StateValidate = "CORE_CONFIG_VALIDATE"
	StateApply    = "CORE_CONFIG_APPLY"
	StateRollback = "CORE_CONFIG_ROLLBACK"
)

type JobStore interface {
	CreateJob(ctx context.Context, kind string, state string, status string, progress int, message string) (store.Job, error)
	MarkJobStarted(ctx context.Context, jobID string) error
	UpdateJob(ctx context.Context, jobID string, state string, status string, progress int, message string, errorCode string, finished bool) error
	AppendJobEvent(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) (store.JobEvent, error)
	LatestJob(ctx context.Context, kind string) (*store.Job, error)
}

type ServiceInstanceStore interface {
	EnsureServiceProfile(ctx context.Context, profileID string, version string, displayName string, expertProtocol string) error
	ActivateServiceInstancesForProfile(ctx context.Context, profileID string, displayName string, listenPort int, configVersion string) ([]store.ServiceInstance, error)
	DeactivateServiceInstancesForProfile(ctx context.Context, profileID string, configVersion string) ([]store.ServiceInstance, error)
}

type Service struct {
	manager *Manager
	store   JobStore
}

type JobResult struct {
	Job       store.Job               `json:"job"`
	Result    Result                  `json:"result"`
	Instances []store.ServiceInstance `json:"instances,omitempty"`
}

func NewService(manager *Manager, appStore JobStore) *Service {
	return &Service{manager: manager, store: appStore}
}

func (s *Service) Apply(ctx context.Context, profileID string) (JobResult, error) {
	if s == nil || s.manager == nil || s.store == nil {
		return JobResult{}, errors.New("service configuration apply is unavailable")
	}
	job, err := s.store.CreateJob(ctx, JobKindServiceConfigApply, StateRender, "queued", 0, "Service configuration apply job created.")
	if err != nil {
		return JobResult{}, err
	}
	if _, err := s.store.AppendJobEvent(ctx, job.ID, JobKindServiceConfigApply, StateRender, "queued", 0, "Service configuration apply job created.", ""); err != nil {
		return JobResult{}, err
	}
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return JobResult{}, err
	}
	if err := s.step(ctx, job.ID, JobKindServiceConfigApply, StateRender, "running", 20, "Rendering backend-owned service configuration.", ""); err != nil {
		return JobResult{}, err
	}
	if err := s.step(ctx, job.ID, JobKindServiceConfigApply, StateValidate, "running", 55, "Validating rendered service configuration.", ""); err != nil {
		return JobResult{}, err
	}
	result, err := s.manager.Apply(ctx, profileID)
	if err != nil {
		_ = s.step(ctx, job.ID, JobKindServiceConfigApply, StateApply, "failed", 100, "Service configuration apply failed; previous configuration was preserved or restored.", "SERVICE_CONFIG_APPLY_FAILED")
		return JobResult{}, err
	}
	instances, err := s.activateInstances(ctx, result)
	if err != nil {
		_ = s.step(ctx, job.ID, JobKindServiceConfigApply, StateApply, "failed", 100, "Service configuration applied, but managed service state could not be updated.", "SERVICE_INSTANCE_SYNC_FAILED")
		return JobResult{}, err
	}
	if err := s.step(ctx, job.ID, JobKindServiceConfigApply, StateApply, "succeeded", 100, "Service configuration applied.", ""); err != nil {
		return JobResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindServiceConfigApply)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{Job: *latest, Result: result, Instances: instances}, nil
}

func (s *Service) Rollback(ctx context.Context, profileID string) (JobResult, error) {
	if s == nil || s.manager == nil || s.store == nil {
		return JobResult{}, errors.New("service configuration rollback is unavailable")
	}
	job, err := s.store.CreateJob(ctx, JobKindServiceConfigRollback, StateRollback, "queued", 0, "Service configuration rollback job created.")
	if err != nil {
		return JobResult{}, err
	}
	if _, err := s.store.AppendJobEvent(ctx, job.ID, JobKindServiceConfigRollback, StateRollback, "queued", 0, "Service configuration rollback job created.", ""); err != nil {
		return JobResult{}, err
	}
	if err := s.store.MarkJobStarted(ctx, job.ID); err != nil {
		return JobResult{}, err
	}
	if err := s.step(ctx, job.ID, JobKindServiceConfigRollback, StateRollback, "running", 50, "Restoring previous validated service configuration.", ""); err != nil {
		return JobResult{}, err
	}
	result, err := s.manager.Rollback(ctx, profileID)
	if err != nil {
		_ = s.step(ctx, job.ID, JobKindServiceConfigRollback, StateRollback, "failed", 100, "Service configuration rollback failed.", "SERVICE_CONFIG_ROLLBACK_FAILED")
		return JobResult{}, err
	}
	instances, err := s.deactivateInstances(ctx, result)
	if err != nil {
		_ = s.step(ctx, job.ID, JobKindServiceConfigRollback, StateRollback, "failed", 100, "Previous service configuration restored, but managed service state could not be updated.", "SERVICE_INSTANCE_SYNC_FAILED")
		return JobResult{}, err
	}
	if err := s.step(ctx, job.ID, JobKindServiceConfigRollback, StateRollback, "succeeded", 100, "Previous service configuration restored.", ""); err != nil {
		return JobResult{}, err
	}
	latest, err := s.store.LatestJob(ctx, JobKindServiceConfigRollback)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{Job: *latest, Result: result, Instances: instances}, nil
}

func (s *Service) step(ctx context.Context, jobID string, kind string, state string, status string, progress int, message string, errorCode string) error {
	finished := status == "succeeded" || status == "failed"
	if err := s.store.UpdateJob(ctx, jobID, state, status, progress, message, errorCode, finished); err != nil {
		return err
	}
	_, err := s.store.AppendJobEvent(ctx, jobID, kind, state, status, progress, message, errorCode)
	return err
}

func (s *Service) activateInstances(ctx context.Context, result Result) ([]store.ServiceInstance, error) {
	instanceStore, ok := s.store.(ServiceInstanceStore)
	if !ok {
		return nil, nil
	}
	if err := instanceStore.EnsureServiceProfile(ctx, result.ProfileID, result.ProfileVersion, result.ProfileDisplayName, result.ExpertProtocol); err != nil {
		return nil, err
	}
	return instanceStore.ActivateServiceInstancesForProfile(ctx, result.ProfileID, result.ProfileDisplayName, result.ListenPort, result.ConfigVersion)
}

func (s *Service) deactivateInstances(ctx context.Context, result Result) ([]store.ServiceInstance, error) {
	instanceStore, ok := s.store.(ServiceInstanceStore)
	if !ok {
		return nil, nil
	}
	if err := instanceStore.EnsureServiceProfile(ctx, result.ProfileID, result.ProfileVersion, result.ProfileDisplayName, result.ExpertProtocol); err != nil {
		return nil, err
	}
	return instanceStore.DeactivateServiceInstancesForProfile(ctx, result.ProfileID, result.ConfigVersion)
}

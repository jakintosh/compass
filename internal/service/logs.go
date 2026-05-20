package service

import (
	"strings"
	"time"
)

type TaskLog struct {
	ID                 string    `json:"id"`
	CategoryID         string    `json:"category_id"`
	ProjectID          string    `json:"project_id"`
	TaskID             string    `json:"task_id"`
	HoursWorked        float64   `json:"hours_worked"`
	WorkDescription    string    `json:"work_description"`
	CompletionEstimate int       `json:"completion_estimate"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	DeletedAt          time.Time `json:"deleted_at"`
}

type ProjectLog struct {
	ID             string    `json:"id"`
	CategoryID     string    `json:"category_id"`
	ProjectID      string    `json:"project_id"`
	StatusEstimate int       `json:"status_estimate"`
	Confidence     string    `json:"confidence"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      time.Time `json:"deleted_at"`
}

type AddTaskLogInput struct {
	AccountID          string
	TaskID             string
	HoursWorked        float64
	WorkDescription    string
	CompletionEstimate int
	CreatedAt          *time.Time
}

func (in *AddTaskLogInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.WorkDescription = strings.TrimSpace(in.WorkDescription)
}

func (in *AddTaskLogInput) Validate() error {
	if in.AccountID == "" ||
		in.TaskID == "" {
		return ErrInvalidInput
	}
	return validateTaskLogFields(in.HoursWorked, in.CompletionEstimate)
}

func (s *Service) AddTaskLog(
	input AddTaskLogInput,
) (
	*TaskLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	taskLog, err := s.store.AddTaskLog(
		input.AccountID,
		input.TaskID,
		input.HoursWorked,
		input.WorkDescription,
		input.CompletionEstimate,
		input.CreatedAt,
	)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return taskLog, nil
}

type AddProjectLogInput struct {
	AccountID      string
	ProjectID      string
	StatusEstimate int
	Confidence     string
	Note           string
	CreatedAt      *time.Time
}

func (in *AddProjectLogInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.Confidence = strings.TrimSpace(strings.ToLower(in.Confidence))
	in.Note = strings.TrimSpace(in.Note)
}

func (in *AddProjectLogInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" {
		return ErrInvalidInput
	}
	if !validCompletion(in.StatusEstimate) ||
		!validConfidence(in.Confidence) {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) AddProjectLog(
	input AddProjectLogInput,
) (
	*ProjectLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	projectLog, err := s.store.AddProjectLog(
		input.AccountID,
		input.ProjectID,
		input.StatusEstimate,
		input.Confidence,
		input.Note,
		input.CreatedAt,
	)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return projectLog, nil
}

type ListTaskLogsInput struct {
	AccountID string
	TaskID    string
}

func (in *ListTaskLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.TaskID = strings.TrimSpace(in.TaskID)
}

func (in *ListTaskLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.TaskID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListTaskLogs(
	input ListTaskLogsInput,
) (
	[]*TaskLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	taskLogs, err := s.store.GetTaskLogsForTask(input.AccountID, input.TaskID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return taskLogs, nil
}

type ListProjectLogsInput struct {
	AccountID string
	ProjectID string
}

func (in *ListProjectLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
}

func (in *ListProjectLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListProjectLogs(
	input ListProjectLogsInput,
) (
	[]*ProjectLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	projectLogs, err := s.store.GetProjectLogsForProject(input.AccountID, input.ProjectID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return projectLogs, nil
}

type ListProjectTaskLogsInput struct {
	AccountID string
	ProjectID string
}

func (in *ListProjectTaskLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
}

func (in *ListProjectTaskLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListProjectTaskLogs(
	input ListProjectTaskLogsInput,
) (
	[]*TaskLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	taskLogs, err := s.store.GetTaskLogsForProject(input.AccountID, input.ProjectID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return taskLogs, nil
}

type ListCategoryTaskLogsInput struct {
	AccountID  string
	CategoryID string
}

func (in *ListCategoryTaskLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
}

func (in *ListCategoryTaskLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.CategoryID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListCategoryTaskLogs(
	input ListCategoryTaskLogsInput,
) (
	[]*TaskLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	taskLogs, err := s.store.GetTaskLogsForCategory(input.AccountID, input.CategoryID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return taskLogs, nil
}

func validateTaskLogFields(
	hoursWorked float64,
	completionEstimate int,
) error {
	if hoursWorked < 0 {
		return ErrInvalidInput
	}
	if !validCompletion(completionEstimate) {
		return ErrInvalidInput
	}
	return nil
}

func validConfidence(confidence string) bool {
	switch confidence {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

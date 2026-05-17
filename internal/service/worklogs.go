package service

import (
	"strings"
	"time"
)

type WorkLog struct {
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

type AddProjectWorkLogInput struct {
	AccountID          string
	ProjectID          string
	HoursWorked        float64
	WorkDescription    string
	CompletionEstimate int
	CreatedAt          *time.Time
}

func (in *AddProjectWorkLogInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.WorkDescription = strings.TrimSpace(in.WorkDescription)
}

func (in *AddProjectWorkLogInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" {
		return ErrInvalidInput
	}
	return validateWorkLogFields(in.HoursWorked, in.CompletionEstimate)
}

func (s *Service) AddProjectWorkLog(
	input AddProjectWorkLogInput,
) (
	*WorkLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	workLog, err := s.store.AddWorkLogForProject(
		input.AccountID,
		input.ProjectID,
		input.HoursWorked,
		input.WorkDescription,
		input.CompletionEstimate,
		input.CreatedAt,
	)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return workLog, nil
}

type AddTaskWorkLogInput struct {
	AccountID          string
	TaskID             string
	HoursWorked        float64
	WorkDescription    string
	CompletionEstimate int
	CreatedAt          *time.Time
}

func (in *AddTaskWorkLogInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.WorkDescription = strings.TrimSpace(in.WorkDescription)
}

func (in *AddTaskWorkLogInput) Validate() error {
	if in.AccountID == "" ||
		in.TaskID == "" {
		return ErrInvalidInput
	}
	return validateWorkLogFields(in.HoursWorked, in.CompletionEstimate)
}

func (s *Service) AddTaskWorkLog(
	input AddTaskWorkLogInput,
) (
	*WorkLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	workLog, err := s.store.AddWorkLogForTask(
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

	return workLog, nil
}

type ListTaskWorkLogsInput struct {
	AccountID string
	TaskID    string
}

func (in *ListTaskWorkLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.TaskID = strings.TrimSpace(in.TaskID)
}

func (in *ListTaskWorkLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.TaskID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListTaskWorkLogs(
	input ListTaskWorkLogsInput,
) (
	[]*WorkLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	workLogs, err := s.store.GetWorkLogsForTask(input.AccountID, input.TaskID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return workLogs, nil
}

type ListProjectWorkLogsInput struct {
	AccountID string
	ProjectID string
}

func (in *ListProjectWorkLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
}

func (in *ListProjectWorkLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListProjectWorkLogs(
	input ListProjectWorkLogsInput,
) (
	[]*WorkLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	workLogs, err := s.store.GetWorkLogsForProject(input.AccountID, input.ProjectID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return workLogs, nil
}

type ListCategoryWorkLogsInput struct {
	AccountID  string
	CategoryID string
}

func (in *ListCategoryWorkLogsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
}

func (in *ListCategoryWorkLogsInput) Validate() error {
	if in.AccountID == "" ||
		in.CategoryID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListCategoryWorkLogs(
	input ListCategoryWorkLogsInput,
) (
	[]*WorkLog,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	workLogs, err := s.store.GetWorkLogsForCategory(input.AccountID, input.CategoryID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return workLogs, nil
}

func validateWorkLogFields(
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

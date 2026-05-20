package service

import (
	"slices"
	"strings"
	"time"
)

type Task struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	CategoryID   string     `json:"category_id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Completion   int        `json:"completion"`
	Public       bool       `json:"public"`
	ParentPublic bool       `json:"parent_public"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	TaskLogs     []*TaskLog `json:"task_logs,omitempty"`
}

type GetTaskInput struct {
	AccountID string
	ID        string
	Viewer    Viewer
}

func (in *GetTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *GetTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) GetTask(
	input GetTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.GetTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if !input.Viewer.CanViewTask(task) {
		return nil, ErrNotFound
	}

	return task, nil
}

func (s *Service) GetTaskWithLogs(
	input GetTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.GetTask(input)
	if err != nil {
		return nil, err
	}

	taskLogs, err := s.store.GetTaskLogsForTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}
	task.TaskLogs = taskLogs

	return task, nil
}

type CreateTaskInput struct {
	AccountID string
	ProjectID string
	Name      string
}

func (in *CreateTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.Name = strings.TrimSpace(in.Name)
}

func (in *CreateTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" ||
		in.Name == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) CreateTask(
	input CreateTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.AddTask(input.AccountID, input.ProjectID, input.Name)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type UpdateTaskInput struct {
	AccountID   string
	ID          string
	Name        *string
	Description *string
	Status      *string
	Completion  *int
	Public      *bool
}

func (in *UpdateTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		in.Name = &name
	}
	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		in.Description = &description
	}
}

func (in *UpdateTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	if in.Name != nil && *in.Name == "" {
		return ErrInvalidInput
	}
	if in.Completion != nil && !validCompletion(*in.Completion) {
		return ErrInvalidInput
	}
	if in.Status != nil && !validStatus(*in.Status) {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) UpdateTask(
	input UpdateTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.GetTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if input.Name != nil {
		task.Name = *input.Name
	}
	if input.Description != nil {
		task.Description = *input.Description
	}
	if input.Completion != nil {
		task.Completion = *input.Completion
	}
	if input.Public != nil {
		task.Public = *input.Public
	}
	if input.Status != nil {
		task.Status = *input.Status
	}

	task, err = s.store.UpdateTask(input.AccountID, task)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type MoveTaskInput struct {
	AccountID       string
	ID              string
	TargetProjectID string
	TargetIndex     int
}

func (in *MoveTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
	in.TargetProjectID = strings.TrimSpace(in.TargetProjectID)
}

func (in *MoveTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" ||
		in.TargetProjectID == "" ||
		in.TargetIndex < 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) MoveTask(
	input MoveTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.MoveTask(input.AccountID, input.ID, input.TargetProjectID, input.TargetIndex)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type ArchiveTaskInput struct {
	AccountID string
	ID        string
}

func (in *ArchiveTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *ArchiveTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ArchiveTask(
	input ArchiveTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.ArchiveTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type RestoreTaskInput struct {
	AccountID string
	ID        string
}

func (in *RestoreTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *RestoreTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) RestoreTask(
	input RestoreTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.RestoreTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type DeleteTaskInput struct {
	AccountID string
	ID        string
}

func (in *DeleteTaskInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *DeleteTaskInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) DeleteTask(
	input DeleteTaskInput,
) (
	*Task,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	task, err := s.store.DeleteTask(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return task, nil
}

type ReorderTasksInput struct {
	AccountID string
	ProjectID string
	TaskIDs   []string
}

func (in *ReorderTasksInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	for i, id := range in.TaskIDs {
		in.TaskIDs[i] = strings.TrimSpace(id)
	}
}

func (in *ReorderTasksInput) Validate() error {
	if in.AccountID == "" ||
		in.ProjectID == "" ||
		len(in.TaskIDs) == 0 {
		return ErrInvalidInput
	}
	if slices.Contains(in.TaskIDs, "") {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ReorderTasks(
	input ReorderTasksInput,
) error {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}

	return mapStoreError(s.store.ReorderTasks(input.AccountID, input.ProjectID, input.TaskIDs))
}

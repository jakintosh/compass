package service

import (
	"slices"
	"strings"
)

type Project struct {
	ID           string     `json:"id"`
	CategoryID   string     `json:"category_id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Completion   int        `json:"completion"`
	Public       bool       `json:"public"`
	ParentPublic bool       `json:"parent_public"`
	Tasks        []*Task    `json:"tasks"`
	WorkLogs     []*WorkLog `json:"work_logs,omitempty"`
}

type GetProjectInput struct {
	AccountID string
	ID        string
	Viewer    Viewer
}

func (in *GetProjectInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *GetProjectInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) GetProject(
	input GetProjectInput,
) (
	*Project,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	project, err := s.store.GetProject(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if !input.Viewer.CanViewProject(project) {
		return nil, ErrNotFound
	}

	return project, nil
}

func (s *Service) GetProjectWithWorkLogs(
	input GetProjectInput,
) (
	*Project,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	project, err := s.GetProject(input)
	if err != nil {
		return nil, err
	}

	workLogs, err := s.store.GetWorkLogsForProject(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}
	project.WorkLogs = workLogs

	return project, nil
}

type CreateProjectInput struct {
	AccountID  string
	CategoryID string
	Name       string
}

func (in *CreateProjectInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
	in.Name = strings.TrimSpace(in.Name)
}

func (in *CreateProjectInput) Validate() error {
	if in.AccountID == "" ||
		in.CategoryID == "" ||
		in.Name == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) CreateProject(
	input CreateProjectInput,
) (
	*Project,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	project, err := s.store.AddProject(input.AccountID, input.CategoryID, input.Name)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return project, nil
}

type UpdateProjectInput struct {
	AccountID   string
	ID          string
	Name        *string
	Description *string
	Completion  *int
	Public      *bool
}

func (in *UpdateProjectInput) Normalize() {
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

func (in *UpdateProjectInput) Validate() error {
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
	return nil
}

func (s *Service) UpdateProject(
	input UpdateProjectInput,
) (
	*Project,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	project, err := s.store.GetProject(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if input.Name != nil {
		project.Name = *input.Name
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	if input.Completion != nil {
		project.Completion = *input.Completion
	}
	if input.Public != nil {
		project.Public = *input.Public
	}

	project, err = s.store.UpdateProject(input.AccountID, project)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return project, nil
}

type DeleteProjectInput struct {
	AccountID string
	ID        string
}

func (in *DeleteProjectInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *DeleteProjectInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) DeleteProject(
	input DeleteProjectInput,
) (
	*Project,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	project, err := s.store.DeleteProject(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)

	}
	return project, nil
}

type ReorderProjectsInput struct {
	AccountID  string
	CategoryID string
	ProjectIDs []string
}

func (in *ReorderProjectsInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
	for i, id := range in.ProjectIDs {
		in.ProjectIDs[i] = strings.TrimSpace(id)
	}
}

func (in *ReorderProjectsInput) Validate() error {
	if in.AccountID == "" ||
		in.CategoryID == "" ||
		len(in.ProjectIDs) == 0 {
		return ErrInvalidInput
	}
	if slices.Contains(in.ProjectIDs, "") {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ReorderProjects(
	input ReorderProjectsInput,
) error {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	return mapStoreError(s.store.ReorderProjects(input.AccountID, input.CategoryID, input.ProjectIDs))
}

func validCompletion(
	completion int,
) bool {
	return completion >= 0 && completion <= 100
}

package service

import (
	"slices"
	"strings"
)

type Category struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Public      bool       `json:"public"`
	Projects    []*Project `json:"projects"`
	WorkLogs    []*WorkLog `json:"work_logs,omitempty"`
}

func (c *Category) AverageCompletion() int {
	if len(c.Projects) == 0 {
		return 0
	}
	sum := 0
	for _, p := range c.Projects {
		sum += p.Completion
	}
	return sum / len(c.Projects)
}

type ListCategoriesInput struct {
	AccountID string
	Viewer    Viewer
}

func (in *ListCategoriesInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
}

func (in *ListCategoriesInput) Validate() error {
	if in.AccountID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListCategories(
	input ListCategoriesInput,
) (
	[]*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cats, err := s.store.GetCategories(input.AccountID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if !input.Viewer.CanWrite {
		cats = FilterPublicCategories(cats)
	}

	return cats, nil
}

type GetCategoryInput struct {
	AccountID string
	ID        string
	Viewer    Viewer
}

func (in *GetCategoryInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *GetCategoryInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) GetCategory(
	input GetCategoryInput,
) (
	*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cat, err := s.store.GetCategory(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if !input.Viewer.CanViewCategory(cat) {
		return nil, ErrNotFound
	}

	return cat, nil
}

func (s *Service) GetCategoryWithWorkLogs(
	input GetCategoryInput,
) (
	*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cat, err := s.GetCategory(input)
	if err != nil {
		return nil, err
	}

	workLogs, err := s.store.GetWorkLogsForCategory(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}
	cat.WorkLogs = workLogs

	return cat, nil
}

type CreateCategoryInput struct {
	AccountID string
	Name      string
}

func (in *CreateCategoryInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.Name = strings.TrimSpace(in.Name)
}

func (in *CreateCategoryInput) Validate() error {
	if in.AccountID == "" ||
		in.Name == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) CreateCategory(
	input CreateCategoryInput,
) (
	*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cat, err := s.store.AddCategory(input.AccountID, input.Name)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return cat, nil
}

type UpdateCategoryInput struct {
	AccountID   string
	ID          string
	Name        *string
	Description *string
	Public      *bool
}

func (in *UpdateCategoryInput) Normalize() {
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

func (in *UpdateCategoryInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	if in.Name != nil && *in.Name == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) UpdateCategory(
	input UpdateCategoryInput,
) (
	*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cat, err := s.store.GetCategory(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	if input.Name != nil {
		cat.Name = *input.Name
	}
	if input.Description != nil {
		cat.Description = *input.Description
	}
	if input.Public != nil {
		cat.Public = *input.Public
	}

	cat, err = s.store.UpdateCategory(input.AccountID, cat)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return cat, nil
}

type DeleteCategoryInput struct {
	AccountID string
	ID        string
}

func (in *DeleteCategoryInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ID = strings.TrimSpace(in.ID)
}

func (in *DeleteCategoryInput) Validate() error {
	if in.AccountID == "" ||
		in.ID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) DeleteCategory(
	input DeleteCategoryInput,
) (
	*Category,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	cat, err := s.store.DeleteCategory(input.AccountID, input.ID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return cat, nil
}

type ReorderCategoriesInput struct {
	AccountID string
	IDs       []string
}

func (in *ReorderCategoriesInput) Normalize() {
	in.AccountID = strings.TrimSpace(in.AccountID)
	for i, id := range in.IDs {
		in.IDs[i] = strings.TrimSpace(id)
	}
}

func (in *ReorderCategoriesInput) Validate() error {
	if in.AccountID == "" ||
		len(in.IDs) == 0 {
		return ErrInvalidInput
	}
	if slices.Contains(in.IDs, "") {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ReorderCategories(
	input ReorderCategoriesInput,
) error {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	return mapStoreError(s.store.ReorderCategories(input.AccountID, input.IDs))
}

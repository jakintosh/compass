package store

import (
	"database/sql"
	"fmt"
	"log"

	"git.sr.ht/~jakintosh/todo/internal/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		collapsed BOOLEAN DEFAULT 0,
		sort_order INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		category_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		completion INTEGER DEFAULT 0,
		sort_order INTEGER DEFAULT 0,
		FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS subtasks (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		name TEXT NOT NULL,
		completion INTEGER DEFAULT 0,
		sort_order INTEGER DEFAULT 0,
		FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) GetCategories() ([]*domain.Category, error) {
	rows, err := s.db.Query(`SELECT id, name, collapsed FROM categories ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Collapsed); err != nil {
			return nil, err
		}
		c.Tasks = []*domain.Task{} // Initialize slice
		categories = append(categories, &c)
	}

	// Now fetch tasks for each category
	// This makes N+1 queries but is simple. For a todo list it's fine.
	// Optimization: Fetch all tasks and distribute them.
	// Let's do the optimization to be cleaner.

	taskRows, err := s.db.Query(`SELECT id, category_id, name, description, completion FROM tasks ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer taskRows.Close()

	tasksByCat := make(map[string][]*domain.Task)
	var allTasks []*domain.Task

	for taskRows.Next() {
		var t domain.Task
		if err := taskRows.Scan(&t.ID, &t.CategoryID, &t.Name, &t.Description, &t.Completion); err != nil {
			return nil, err
		}
		t.Subtasks = []*domain.Subtask{}
		tasksByCat[t.CategoryID] = append(tasksByCat[t.CategoryID], &t)
		allTasks = append(allTasks, &t)
	}

	// Fetch all subtasks
	subRows, err := s.db.Query(`SELECT id, task_id, name, completion FROM subtasks ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer subRows.Close()

	subsByTask := make(map[string][]*domain.Subtask)
	for subRows.Next() {
		var sub domain.Subtask
		if err := subRows.Scan(&sub.ID, &sub.TaskID, &sub.Name, &sub.Completion); err != nil {
			return nil, err
		}
		subsByTask[sub.TaskID] = append(subsByTask[sub.TaskID], &sub)
	}

	// Assemble
	for _, t := range allTasks {
		if subs, ok := subsByTask[t.ID]; ok {
			t.Subtasks = subs
		}
	}

	for _, c := range categories {
		if tasks, ok := tasksByCat[c.ID]; ok {
			c.Tasks = tasks
		}
	}

	return categories, nil
}

func (s *SQLiteStore) GetCategory(id string) (*domain.Category, error) {
	var c domain.Category
	err := s.db.QueryRow(`SELECT id, name, collapsed FROM categories WHERE id = ?`, id).Scan(&c.ID, &c.Name, &c.Collapsed)
	if err != nil {
		return nil, err
	}

	// We need to fill tasks too if we want full object, but does the app rely on it?
	// The interface implies we return the Category struct which has Tasks field.
	// In the memory store we return the object which has the tasks.
	// So yes, we should probably fetch tasks.
	// However, usually GetCategory might just be for metadata.
	// Let's check usage. `handleToggleCollapseCategory` just flips a bool and saves.
	// `handleCreateCategory` returns a new category.
	// To be safe, let's load tasks.

	// Actually for `GetCategory` in `handleToggleCollapseCategory`, we only need the struct fields.
	// But let's act like a proper object store.

	tasks, err := s.getTasksForCategory(c.ID)
	if err != nil {
		return nil, err
	}
	c.Tasks = tasks
	return &c, nil
}

func (s *SQLiteStore) getTasksForCategory(catID string) ([]*domain.Task, error) {
	rows, err := s.db.Query(`SELECT id, category_id, name, description, completion FROM tasks WHERE category_id = ? ORDER BY sort_order ASC`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.CategoryID, &t.Name, &t.Description, &t.Completion); err != nil {
			return nil, err
		}

		subs, err := s.getSubtasksForTask(t.ID)
		if err != nil {
			return nil, err
		}
		t.Subtasks = subs

		tasks = append(tasks, &t)
	}
	return tasks, nil
}

func (s *SQLiteStore) getSubtasksForTask(taskID string) ([]*domain.Subtask, error) {
	rows, err := s.db.Query(`SELECT id, task_id, name, completion FROM subtasks WHERE task_id = ? ORDER BY sort_order ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subtask
	for rows.Next() {
		var sub domain.Subtask
		if err := rows.Scan(&sub.ID, &sub.TaskID, &sub.Name, &sub.Completion); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, nil
}

func (s *SQLiteStore) AddCategory(name string) (*domain.Category, error) {
	id := uuid.NewString()
	// Get max sort_order
	var maxOrder sql.NullInt64
	s.db.QueryRow("SELECT MAX(sort_order) FROM categories").Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	_, err := s.db.Exec(`INSERT INTO categories (id, name, sort_order) VALUES (?, ?, ?)`, id, name, order)
	if err != nil {
		return nil, err
	}

	return &domain.Category{
		ID:    id,
		Name:  name,
		Tasks: []*domain.Task{},
	}, nil
}

func (s *SQLiteStore) UpdateCategory(cat *domain.Category) error {
	_, err := s.db.Exec(`UPDATE categories SET name = ?, collapsed = ? WHERE id = ?`, cat.Name, cat.Collapsed, cat.ID)
	return err
}

func (s *SQLiteStore) DeleteCategory(id string) error {
	// Cascade should handle tasks/subtasks
	result, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

func (s *SQLiteStore) ReorderCategories(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE categories SET sort_order = ? WHERE id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetTask(id string) (*domain.Task, error) {
	var t domain.Task
	err := s.db.QueryRow(`SELECT id, category_id, name, description, completion FROM tasks WHERE id = ?`, id).Scan(&t.ID, &t.CategoryID, &t.Name, &t.Description, &t.Completion)
	if err != nil {
		return nil, err
	}
	subs, err := s.getSubtasksForTask(t.ID)
	if err != nil {
		return nil, err
	}
	t.Subtasks = subs
	return &t, nil
}

func (s *SQLiteStore) AddTask(catID string, name string) (*domain.Task, error) {
	id := uuid.NewString()

	var maxOrder sql.NullInt64
	s.db.QueryRow("SELECT MAX(sort_order) FROM tasks WHERE category_id = ?", catID).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	if _, err := s.db.Exec(`INSERT INTO tasks (id, category_id, name, sort_order) VALUES (?, ?, ?, ?)`, id, catID, name, order); err != nil {
		return nil, err
	}
	return &domain.Task{
		ID:         id,
		CategoryID: catID,
		Name:       name,
		Subtasks:   []*domain.Subtask{},
	}, nil
}

func (s *SQLiteStore) UpdateTask(task *domain.Task) error {
	_, err := s.db.Exec(`UPDATE tasks SET name = ?, description = ?, completion = ? WHERE id = ?`, task.Name, task.Description, task.Completion, task.ID)
	return err
}

func (s *SQLiteStore) DeleteTask(id string) error {
	result, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

func (s *SQLiteStore) MoveTask(taskID string, newCatID string, newIndex int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get current category to reorder old siblings? No strictly necessary if we rely on simple incrementing or relative order.
	// But gaps are fine.

	// 2. Update the task
	// Need to make space in the new category.
	// Shift everything >= newIndex up by 1.
	if _, err := tx.Exec(`UPDATE tasks SET sort_order = sort_order + 1 WHERE category_id = ? AND sort_order >= ?`, newCatID, newIndex); err != nil {
		return err
	}

	if _, err := tx.Exec(`UPDATE tasks SET category_id = ?, sort_order = ? WHERE id = ?`, newCatID, newIndex, taskID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) ReorderTasks(catID string, taskIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range taskIDs {
		// Verify task belongs to category? Or just trust the ID defaults?
		// Ideally verify.
		if _, err := tx.Exec(`UPDATE tasks SET sort_order = ? WHERE id = ? AND category_id = ?`, i, id, catID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetSubtask(id string) (*domain.Subtask, error) {
	var sub domain.Subtask
	err := s.db.QueryRow(`SELECT id, task_id, name, completion FROM subtasks WHERE id = ?`, id).Scan(&sub.ID, &sub.TaskID, &sub.Name, &sub.Completion)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *SQLiteStore) AddSubtask(taskID string, name string) (*domain.Subtask, error) {
	id := uuid.NewString()

	var maxOrder sql.NullInt64
	s.db.QueryRow("SELECT MAX(sort_order) FROM subtasks WHERE task_id = ?", taskID).Scan(&maxOrder)
	order := int(maxOrder.Int64) + 1

	if _, err := s.db.Exec(`INSERT INTO subtasks (id, task_id, name, sort_order) VALUES (?, ?, ?, ?)`, id, taskID, name, order); err != nil {
		return nil, err
	}

	// Update parent task completion logic is currently in memory store...
	// The memory store does: `t.UpdateCompletion()`.
	// The domain model has the logic.
	// But wait, the domain model needs the subtasks to calculate completion.
	// In the memory store we held the objects. Here we don't.
	// So we need to fetch all subtasks for the task, calculate completion, and save the task.

	if err := s.updateTaskCompletionInternal(taskID); err != nil {
		// Log error but don't fail the add? Or fail?
		log.Printf("Failed to update task completion: %v", err)
	}

	return &domain.Subtask{
		ID:     id,
		TaskID: taskID,
		Name:   name,
	}, nil
}

func (s *SQLiteStore) UpdateSubtask(sub *domain.Subtask) error {
	_, err := s.db.Exec(`UPDATE subtasks SET name = ?, completion = ? WHERE id = ?`, sub.Name, sub.Completion, sub.ID)
	if err != nil {
		return err
	}

	// Need to fetch the subtask to get task_id IF it wasn't populated in the input struct.
	// But typically `UpdateSubtask` receives a struct that might be incomplete if constructed from form?
	// The form handler does `s.store.GetSubtask(id)` first, then updates fields.
	// So `sub` has `TaskID`.

	// But wait, `GetSubtask` in `server.go` calls our `GetSubtask`, which sets `TaskID`.
	// So `sub.TaskID` should be set.

	if sub.TaskID != "" {
		return s.updateTaskCompletionInternal(sub.TaskID)
	}
	// If missing, we have to look it up.
	var taskID string
	if err := s.db.QueryRow("SELECT task_id FROM subtasks WHERE id = ?", sub.ID).Scan(&taskID); err == nil {
		return s.updateTaskCompletionInternal(taskID)
	}

	return nil
}

func (s *SQLiteStore) DeleteSubtask(id string) error {
	var taskID string
	s.db.QueryRow("SELECT task_id FROM subtasks WHERE id = ?", id).Scan(&taskID)

	result, err := s.db.Exec(`DELETE FROM subtasks WHERE id = ?`, id)
	if err != nil {
		return err
	}

	if taskID != "" {
		s.updateTaskCompletionInternal(taskID)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("subtask not found")
	}
	return nil
}

func (s *SQLiteStore) ReorderSubtasks(taskID string, subIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range subIDs {
		if _, err := tx.Exec(`UPDATE subtasks SET sort_order = ? WHERE id = ? AND task_id = ?`, i, id, taskID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) updateTaskCompletionInternal(taskID string) error {
	subs, err := s.getSubtasksForTask(taskID)
	if err != nil {
		return err
	}

	// Create a dummy task to use the domain logic
	t := &domain.Task{Subtasks: subs}
	t.UpdateCompletion()

	_, err = s.db.Exec(`UPDATE tasks SET completion = ? WHERE id = ?`, t.Completion, taskID)
	return err
}

func (s *SQLiteStore) Seed() {
	// Check if empty
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count)
	if count > 0 {
		return
	}

	cat1, _ := s.AddCategory("Work")
	task1, _ := s.AddTask(cat1.ID, "Finish Report")
	task1.Completion = 20
	s.UpdateTask(task1)

	cat2, _ := s.AddCategory("Personal")
	task2, _ := s.AddTask(cat2.ID, "Buy Groceries")

	sub1, _ := s.AddSubtask(task2.ID, "Milk")
	sub1.Completion = 0
	s.UpdateSubtask(sub1)

	sub2, _ := s.AddSubtask(task2.ID, "Eggs")
	sub2.Completion = 100
	s.UpdateSubtask(sub2)
}

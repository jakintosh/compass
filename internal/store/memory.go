package store

import (
	"errors"
	"sync"

	"git.sr.ht/~jakintosh/todo/internal/domain"
	"github.com/google/uuid"
)

type InMemoryStore struct {
	mu         sync.RWMutex
	categories []*domain.Category
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		categories: []*domain.Category{},
	}
}

func (s *InMemoryStore) GetCategories() ([]*domain.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a deep copy or just structure copy to avoid race conditions if caller modifies?
	// For simplicity in this in-memory mock, returning pointer is risky but usually okay for simple single-process app.
	// But let's return the slice as is, since we are returning pointers to structs.
	// The implementation plan says "Simple", so we won't over-engineer concurrency safety beyond the mutex on the map/slice.
	return s.categories, nil
}

func (s *InMemoryStore) GetCategory(id string) (*domain.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.categories {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, errors.New("category not found")
}

func (s *InMemoryStore) AddCategory(name string) (*domain.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cat := &domain.Category{
		ID:    uuid.NewString(),
		Name:  name,
		Tasks: []*domain.Task{},
	}
	s.categories = append(s.categories, cat)
	return cat, nil
}

func (s *InMemoryStore) UpdateCategory(cat *domain.Category) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.categories {
		if c.ID == cat.ID {
			s.categories[i] = cat
			return nil
		}
	}
	return errors.New("category not found")
}

func (s *InMemoryStore) DeleteCategory(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.categories {
		if c.ID == id {
			s.categories = append(s.categories[:i], s.categories[i+1:]...)
			return nil
		}
	}
	return errors.New("category not found")
}

func (s *InMemoryStore) ReorderCategories(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newOrder := make([]*domain.Category, 0, len(s.categories))
	lookup := make(map[string]*domain.Category)
	for _, c := range s.categories {
		lookup[c.ID] = c
	}

	for _, id := range ids {
		if cat, ok := lookup[id]; ok {
			newOrder = append(newOrder, cat)
			delete(lookup, id)
		}
	}

	// Append any remaining categories appropriately (if any were missed in ids list)
	// Theoretically shouldn't happen if UI sends full list, but good for safety.
	for _, c := range s.categories {
		if _, ok := lookup[c.ID]; ok { // if still in lookup, it wasn't in the new order
			newOrder = append(newOrder, c)
		}
	}

	s.categories = newOrder
	return nil
}

func (s *InMemoryStore) GetTask(id string) (*domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.categories {
		for _, t := range c.Tasks {
			if t.ID == id {
				return t, nil
			}
		}
	}
	return nil, errors.New("task not found")
}

func (s *InMemoryStore) AddTask(catID string, name string) (*domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		if c.ID == catID {
			task := &domain.Task{
				ID:         uuid.NewString(),
				CategoryID: catID,
				Name:       name,
				Subtasks:   []*domain.Subtask{},
			}
			c.Tasks = append(c.Tasks, task)
			return task, nil
		}
	}
	return nil, errors.New("category not found")
}

func (s *InMemoryStore) UpdateTask(task *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Since task could be in any category, we find it and replace it.
	// But wait, the task struct has CategoryID.

	for _, c := range s.categories {
		if c.ID == task.CategoryID {
			for i, t := range c.Tasks {
				if t.ID == task.ID {
					c.Tasks[i] = task
					return nil
				}
			}
		}
	}
	return errors.New("task not found in its specified category")
}

func (s *InMemoryStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		for i, t := range c.Tasks {
			if t.ID == id {
				c.Tasks = append(c.Tasks[:i], c.Tasks[i+1:]...)
				return nil
			}
		}
	}
	return errors.New("task not found")
}

func (s *InMemoryStore) MoveTask(taskID string, newCatID string, newIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var taskToMove *domain.Task

	// 1. Remove from old location
	for _, c := range s.categories {
		for i, t := range c.Tasks {
			if t.ID == taskID {
				taskToMove = t
				// Remove
				c.Tasks = append(c.Tasks[:i], c.Tasks[i+1:]...)
				break
			}
		}
		if taskToMove != nil {
			break
		}
	}

	if taskToMove == nil {
		return errors.New("task not found")
	}

	// 2. Add to new location
	taskToMove.CategoryID = newCatID

	for _, c := range s.categories {
		if c.ID == newCatID {
			// Insert at newIndex
			if newIndex < 0 {
				newIndex = 0
			}
			if newIndex > len(c.Tasks) {
				newIndex = len(c.Tasks)
			}

			c.Tasks = append(c.Tasks[:newIndex], append([]*domain.Task{taskToMove}, c.Tasks[newIndex:]...)...)
			return nil
		}
	}

	return errors.New("target category not found")
}

func (s *InMemoryStore) ReorderTasks(catID string, taskIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		if c.ID == catID {
			newTasks := make([]*domain.Task, 0, len(c.Tasks))
			lookup := make(map[string]*domain.Task)
			for _, t := range c.Tasks {
				lookup[t.ID] = t
			}

			for _, id := range taskIDs {
				if t, ok := lookup[id]; ok {
					newTasks = append(newTasks, t)
					delete(lookup, id)
				}
			}

			// Append leftovers
			for _, t := range c.Tasks {
				if _, ok := lookup[t.ID]; ok {
					newTasks = append(newTasks, t)
				}
			}
			c.Tasks = newTasks
			return nil
		}
	}
	return errors.New("category not found")
}

func (s *InMemoryStore) GetSubtask(id string) (*domain.Subtask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.categories {
		for _, t := range c.Tasks {
			for _, sub := range t.Subtasks {
				if sub.ID == id {
					return sub, nil
				}
			}
		}
	}
	return nil, errors.New("subtask not found")
}

func (s *InMemoryStore) AddSubtask(taskID string, name string) (*domain.Subtask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		for _, t := range c.Tasks {
			if t.ID == taskID {
				sub := &domain.Subtask{
					ID:   uuid.NewString(),
					Name: name,
				}
				t.Subtasks = append(t.Subtasks, sub)
				t.UpdateCompletion() // Recalculate
				return sub, nil
			}
		}
	}
	return nil, errors.New("parent task not found")
}

func (s *InMemoryStore) UpdateSubtask(sub *domain.Subtask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		for _, t := range c.Tasks {
			for i, sItem := range t.Subtasks {
				if sItem.ID == sub.ID {
					t.Subtasks[i] = sub
					t.UpdateCompletion()
					return nil
				}
			}
		}
	}
	return errors.New("subtask not found")
}

func (s *InMemoryStore) DeleteSubtask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		for _, t := range c.Tasks {
			for i, sub := range t.Subtasks {
				if sub.ID == id {
					t.Subtasks = append(t.Subtasks[:i], t.Subtasks[i+1:]...)
					t.UpdateCompletion()
					return nil
				}
			}
		}
	}
	return errors.New("subtask not found")
}

func (s *InMemoryStore) ReorderSubtasks(taskID string, subIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.categories {
		for _, t := range c.Tasks {
			if t.ID == taskID {
				newSubs := make([]*domain.Subtask, 0, len(t.Subtasks))
				lookup := make(map[string]*domain.Subtask)
				for _, sub := range t.Subtasks {
					lookup[sub.ID] = sub
				}

				for _, id := range subIDs {
					if sub, ok := lookup[id]; ok {
						newSubs = append(newSubs, sub)
						delete(lookup, id)
					}
				}

				for _, sub := range t.Subtasks {
					if _, ok := lookup[sub.ID]; ok {
						newSubs = append(newSubs, sub)
					}
				}
				t.Subtasks = newSubs
				return nil
			}
		}
	}
	return errors.New("parent task not found")
}

func (s *InMemoryStore) Seed() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cat1 := &domain.Category{ID: uuid.NewString(), Name: "Work", Tasks: []*domain.Task{}}
	task1 := &domain.Task{ID: uuid.NewString(), CategoryID: cat1.ID, Name: "Finish Report", Completion: 20}
	cat1.Tasks = append(cat1.Tasks, task1)

	cat2 := &domain.Category{ID: uuid.NewString(), Name: "Personal", Tasks: []*domain.Task{}}
	task2 := &domain.Task{ID: uuid.NewString(), CategoryID: cat2.ID, Name: "Buy Groceries", Completion: 0}

	sub1 := &domain.Subtask{ID: uuid.NewString(), Name: "Milk", Completion: 0}
	sub2 := &domain.Subtask{ID: uuid.NewString(), Name: "Eggs", Completion: 100}
	task2.Subtasks = append(task2.Subtasks, sub1, sub2)
	task2.UpdateCompletion() // Should be 50

	cat2.Tasks = append(cat2.Tasks, task2)

	s.categories = append(s.categories, cat1, cat2)
}

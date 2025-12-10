# Vibes Todo

A minimal, single-page todo application that uses sliders instead of checkboxes. Each task represents progress on a continuum from 0% to 100%, rather than a binary done/not-done state.

## Key Features

### Tasks as Progress
Every task is a slider ranging from 0% to 100%. This lets you track gradual progress rather than just marking things complete.

### Hierarchical Organization
- **Categories**: Group related tasks together
- **Tasks**: Individual items you're working on
- **Subtasks**: Break down complex tasks into smaller pieces

When a task has subtasks, it no longer has its own slider. Instead, its completion percentage is automatically calculated as the average of all its subtasks.

### Flexible Management
- **Reorder anything**: Drag and drop categories, tasks, and subtasks to organize them however you like
- **Move tasks between categories**: Tasks can be dragged from one category to another
- **Collapse categories**: Hide tasks you're not currently focused on
- **Task details**: Click any task to view and edit its name and description

## Running the Application

```bash
make run
```

The application will be available at `http://localhost:8080`.

## Usage

1. **Create a category** using the "New Category +" button in the header
2. **Add tasks** using the "Add a task" link within any category
3. **Adjust progress** by dragging the slider for each task
4. **View details** by clicking on any task name
5. **Add subtasks** from the task details view
6. **Reorder** by dragging items using their handle (visible on hover)

## Philosophy

This app makes no assumptions about what completion means for your tasks. The slider is deliberately abstract—100% simply means "done" in whatever way makes sense to you. Everything in between is yours to define.

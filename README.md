# Compass

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

For local development without a Consent server, run Compass in dev auth mode:

```bash
go run ./cmd/compass --dev
```

For a production Consent integration, register this app from its well-known
manifest at `https://your-compass-host/.well-known/consent-integration`, then
start Compass with:

```bash
CONSENT_URL=https://consent.example.com \
CONSENT_PUBKEY="$(cat /path/to/consent/public.pem)" \
PUBLIC_URL=https://your-compass-host \
CONSENT_INTEGRATION=compass \
go run ./cmd/compass
```

`PUBLIC_URL` is used to publish the manifest, callback URL, logo URL, and JWT
audience. The login URL requests the `identity` and `profile` Consent scopes so
Compass can cache each user's handle for tenant URLs like
`https://your-compass-host/{handle}/`. The default integration name is
`compass`.

## Usage

1. **Create a category** using the "New Category +" button in the header
2. **Add tasks** using the "Add a task" link within any category
3. **Adjust progress** by dragging the slider for each task
4. **View details** by clicking on any task name
5. **Add subtasks** from the task details view
6. **Reorder** by dragging items using their handle (visible on hover)

## Philosophy

This app makes no assumptions about what completion means for your tasks. The slider is deliberately abstract—100% simply means "done" in whatever way makes sense to you. Everything in between is yours to define.

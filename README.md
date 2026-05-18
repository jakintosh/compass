# Compass

A minimal, single-page planning application that uses sliders instead of checkboxes. Each project or task represents progress on a continuum from 0% to 100%, rather than a binary done/not-done state.

## Key Features

### Progress as a Continuum
Every project and task is a slider ranging from 0% to 100%. This lets you track gradual progress rather than just marking things complete.

### Hierarchical Organization
- **Categories**: Group related projects together
- **Projects**: Larger efforts or outcomes you're working toward
- **Tasks**: Concrete pieces of work inside a project

Projects can contain tasks, and both levels can carry their own progress and work log history.

### Flexible Management
- **Reorder anything**: Drag and drop categories, projects, and tasks to organize them however you like
- **Move projects between categories**: Projects can be dragged from one category to another
- **Collapse categories and projects**: Hide work you're not currently focused on
- **Project and task details**: Click any project or task to view and edit its name and description

## Running the Application

```bash
make run
```

The application will be available at `http://localhost:8080`.

For local development without a Consent server, `make run` runs the Docker image
in dev auth mode. To run the binary directly:

```bash
go run ./cmd/compass serve --dev --data-dir ./data
```

For a production Consent integration, register this app from its well-known
manifest at `https://your-compass-host/.well-known/consent-integration`, then
start Compass with:

```bash
CONSENT_URL=https://consent.example.com \
CONSENT_PUBKEY="$(cat /path/to/consent/public.pem)" \
PUBLIC_URL=https://your-compass-host \
CONSENT_INTEGRATION=compass \
go run ./cmd/compass serve
```

The Docker image defaults to production mode on port 80 and stores runtime state
in `/app/data`, so production containers can provide the same Consent settings
through environment variables:

```bash
docker run --rm \
  -p 8080:80 \
  -v "$PWD/data:/app/data" \
  -e CONSENT_URL=https://consent.example.com \
  -e CONSENT_PUBKEY="$(cat /path/to/consent/public.pem)" \
  -e PUBLIC_URL=https://your-compass-host \
  -e CONSENT_INTEGRATION=compass \
  compass:latest
```

`PUBLIC_URL` is used to publish the manifest, callback URL, logo URL, and JWT
audience. The login URL requests the `identity` and `profile` Consent scopes so
Compass can cache each user's handle for tenant URLs like
`https://your-compass-host/{handle}/`. The default integration name is
`compass`.

## Usage

1. **Create a category** using the "New Category +" button in the header
2. **Add projects** using the "Add a project" link within any category
3. **Add tasks** using the "Add a task" link within any project
4. **Adjust progress** by dragging the slider for each project or task
5. **View details** by clicking on any project or task name
6. **Reorder** by dragging items using their handle (visible on hover)

## Philosophy

This app makes no assumptions about what completion means for your projects or tasks. The slider is deliberately abstract—100% simply means "done" in whatever way makes sense to you. Everything in between is yours to define.

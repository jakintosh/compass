package database

import (
	"database/sql"
	"fmt"

	"git.sr.ht/~jakintosh/compass/internal/service"
	"github.com/google/uuid"
)

func (db *DB) GetCategories(
	accountID string,
) (
	[]*service.Category,
	error,
) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin get categories tx: %w", err)
	}
	defer tx.Rollback()

	categories, err := db.sqlListCategoriesTx(tx, accountID)
	if err != nil {
		return nil, err
	}

	projectsByCat, allProjects, err := db.sqlListProjectsByCategoryTx(tx, accountID)
	if err != nil {
		return nil, err
	}

	tasksByProject, err := db.sqlListTasksByProjectTx(tx, accountID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit get categories tx: %w", err)
	}

	attachTasksToProjects(allProjects, tasksByProject)
	attachProjectsToCategories(categories, projectsByCat)

	return categories, nil
}

func (db *DB) GetCategory(
	accountID string,
	id string,
) (
	*service.Category,
	error,
) {
	row := db.Conn.QueryRow(`
		SELECT id, name, description, public
		FROM categories
		WHERE account_id = ?1 AND id = ?2`,
		accountID,
		id,
	)

	cat, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("get category %q: %w", id, err)
	}

	projects, err := db.getProjectsForCategory(accountID, cat.ID)
	if err != nil {
		return nil, err
	}

	cat.Projects = projects
	return cat, nil
}

func (db *DB) AddCategory(
	accountID string,
	name string,
) (
	*service.Category,
	error,
) {
	id := uuid.NewString()

	var minOrder sql.NullInt64
	if err := db.Conn.QueryRow(`
		SELECT MIN(sort_order)
		FROM categories
		WHERE account_id = ?1`,
		accountID,
	).Scan(&minOrder); err != nil {
		return nil, fmt.Errorf("select category sort order: %w", err)
	}
	order := int(minOrder.Int64) - 1

	row := db.Conn.QueryRow(`
		INSERT INTO categories (id, account_id, name, sort_order)
		VALUES (?1, ?2, ?3, ?4)
		RETURNING id, name, description, public`,
		id,
		accountID,
		name,
		order,
	)

	cat, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("add category: %w", err)
	}

	cat.Projects = []*service.Project{}
	return cat, nil
}

func (db *DB) UpdateCategory(
	accountID string,
	cat *service.Category,
) (
	*service.Category,
	error,
) {
	row := db.Conn.QueryRow(`
		UPDATE categories
		SET name = ?1,
			description = ?2,
			public = ?3
		WHERE account_id = ?4 AND id = ?5
		RETURNING id, name, description, public`,
		cat.Name,
		cat.Description,
		cat.Public,
		accountID,
		cat.ID,
	)

	updated, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("update category %q: %w", cat.ID, err)
	}

	projects, err := db.getProjectsForCategory(accountID, updated.ID)
	if err != nil {
		return nil, err
	}

	updated.Projects = projects
	return updated, nil
}

func (db *DB) DeleteCategory(
	accountID string,
	id string,
) (
	*service.Category,
	error,
) {
	row := db.Conn.QueryRow(`
		DELETE FROM categories
		WHERE account_id = ?1 AND id = ?2
		RETURNING id, name, description, public`,
		accountID,
		id,
	)

	removed, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("delete category %q: %w", id, err)
	}

	return removed, nil
}

func (db *DB) ReorderCategories(
	accountID string,
	ids []string,
) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("begin reorder categories tx: %w", err)
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`
			UPDATE categories
			SET sort_order = ?1
			WHERE account_id = ?2 AND id = ?3`,
			i,
			accountID,
			id,
		); err != nil {
			return fmt.Errorf("reorder category %q: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder categories tx: %w", err)
	}

	return nil
}

func (db *DB) sqlListCategoriesTx(
	tx *sql.Tx,
	accountID string,
) (
	[]*service.Category,
	error,
) {
	rows, err := tx.Query(`
		SELECT id, name, description, public
		FROM categories
		WHERE account_id = ?1
		ORDER BY sort_order ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []*service.Category
	for rows.Next() {
		var cat service.Category
		if err := rows.Scan(
			&cat.ID,
			&cat.Name,
			&cat.Description,
			&cat.Public,
		); err != nil {
			return nil, fmt.Errorf("scan category row: %w", err)
		}
		cat.Projects = []*service.Project{}
		categories = append(categories, &cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category rows: %w", err)
	}

	return categories, nil
}

func (db *DB) sqlListProjectsByCategoryTx(
	tx *sql.Tx,
	accountID string,
) (
	map[string][]*service.Project,
	[]*service.Project,
	error,
) {
	rows, err := tx.Query(`
		SELECT
			t.id,
			t.category_id,
			t.name,
			t.description,
			t.completion,
			t.public,
			c.public AS parent_public
		FROM projects t
		JOIN categories c ON t.category_id = c.id
		WHERE t.account_id = ?1
		ORDER BY t.sort_order ASC`,
		accountID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list projects for categories: %w", err)
	}
	defer rows.Close()

	projectsByCat := make(map[string][]*service.Project)
	var allProjects []*service.Project
	for rows.Next() {
		project, err := scanProjectRows(rows)
		if err != nil {
			return nil, nil, err
		}

		project.Tasks = []*service.Task{}
		projectsByCat[project.CategoryID] = append(projectsByCat[project.CategoryID], project)
		allProjects = append(allProjects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate project rows: %w", err)
	}

	return projectsByCat, allProjects, nil
}

func (db *DB) sqlListTasksByProjectTx(
	tx *sql.Tx,
	accountID string,
) (
	map[string][]*service.Task,
	error,
) {
	rows, err := tx.Query(`
		SELECT
			s.id,
			s.project_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM tasks s
		JOIN projects t ON s.project_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1
		ORDER BY s.sort_order ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks for projects: %w", err)
	}
	defer rows.Close()

	tasksByProject := make(map[string][]*service.Task)
	for rows.Next() {
		task, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasksByProject[task.ProjectID] = append(tasksByProject[task.ProjectID], task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task rows: %w", err)
	}

	return tasksByProject, nil
}

func (db *DB) getProjectsForCategory(
	accountID string,
	catID string,
) (
	[]*service.Project,
	error,
) {
	rows, err := db.Conn.Query(`
		SELECT
			t.id,
			t.category_id,
			t.name,
			t.description,
			t.completion,
			t.public,
			c.public AS parent_public
		FROM projects t
		JOIN categories c ON t.category_id = c.id
		WHERE t.account_id = ?1 AND t.category_id = ?2
		ORDER BY t.sort_order ASC`,
		accountID,
		catID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects for category %q: %w", catID, err)
	}
	defer rows.Close()

	var projects []*service.Project
	for rows.Next() {
		project, err := scanProjectRows(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows: %w", err)
	}

	for _, project := range projects {
		tasks, err := db.getTasksForProject(accountID, project.ID)
		if err != nil {
			return nil, err
		}
		project.Tasks = tasks
	}

	return projects, nil
}

func (db *DB) getTasksForProject(
	accountID string,
	projectID string,
) (
	[]*service.Task,
	error,
) {
	rows, err := db.Conn.Query(`
		SELECT
			s.id,
			s.project_id,
			s.category_id,
			s.name,
			s.description,
			s.completion,
			s.public,
			(c.public AND t.public) AS parent_public
		FROM tasks s
		JOIN projects t ON s.project_id = t.id
		JOIN categories c ON s.category_id = c.id
		WHERE s.account_id = ?1 AND s.project_id = ?2
		ORDER BY s.sort_order ASC`,
		accountID,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks for project %q: %w", projectID, err)
	}
	defer rows.Close()

	var tasks []*service.Task
	for rows.Next() {
		task, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task rows: %w", err)
	}

	return tasks, nil
}

func scanCategory(
	row *sql.Row,
) (
	*service.Category,
	error,
) {
	var cat service.Category
	if err := row.Scan(
		&cat.ID,
		&cat.Name,
		&cat.Description,
		&cat.Public,
	); err != nil {
		return nil, err
	}
	return &cat, nil
}

func attachTasksToProjects(
	projects []*service.Project,
	tasksByProject map[string][]*service.Task,
) {
	for _, project := range projects {
		if tasks, ok := tasksByProject[project.ID]; ok {
			project.Tasks = tasks
		}
	}
}

func attachProjectsToCategories(
	categories []*service.Category,
	projectsByCat map[string][]*service.Project,
) {
	for _, cat := range categories {
		if projects, ok := projectsByCat[cat.ID]; ok {
			cat.Projects = projects
		}
	}
}

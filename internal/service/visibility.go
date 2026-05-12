package service

type Viewer struct {
	CanWrite bool
}

func OwnerViewer() Viewer {
	return Viewer{CanWrite: true}
}

func PublicViewer() Viewer {
	return Viewer{}
}

func FilterPublicCategories(
	cats []*Category,
) []*Category {
	var result []*Category
	for _, c := range cats {
		if !c.Public {
			continue
		}
		var publicProjects []*Project
		for _, p := range c.Projects {
			if !p.Public {
				continue
			}
			var publicTasks []*Task
			for _, t := range p.Tasks {
				if t.Public {
					publicTasks = append(publicTasks, t)
				}
			}
			p.Tasks = publicTasks
			publicProjects = append(publicProjects, p)
		}
		c.Projects = publicProjects
		result = append(result, c)
	}
	return result
}

func (v Viewer) CanViewCategory(
	category *Category,
) bool {
	return v.CanWrite || category.Public
}

func (v Viewer) CanViewProject(
	project *Project,
) bool {
	return v.CanWrite || (project.ParentPublic && project.Public)
}

func (v Viewer) CanViewTask(
	task *Task,
) bool {
	return v.CanWrite || (task.ParentPublic && task.Public)
}

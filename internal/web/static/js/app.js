document.addEventListener('htmx:load', function (evt) {
    // Initialize Sortable for Categories
    let categoriesList = document.getElementById('categories-list');
    if (categoriesList && !categoriesList.sortableInitialized) {
        new Sortable(categoriesList, {
            animation: 150,
            handle: '.handle',
            ghostClass: 'ghost',
            onEnd: function (evt) {
                let ids = this.toArray();
                htmx.ajax('POST', '/categories/reorder', {
                    values: { id: ids },
                    swap: 'none'
                });
            }
        });
        categoriesList.sortableInitialized = true;
    }

    // Initialize Sortable for Tasks within Categories
    document.querySelectorAll('.tasks-list').forEach(function (el) {
        if (!el.sortableInitialized) {
            new Sortable(el, {
                group: 'tasks',
                animation: 150,
                handle: '.task-handle',
                ghostClass: 'ghost',
                onEnd: function (evt) {
                    let taskId = evt.item.getAttribute('data-id');
                    let toCatId = evt.to.getAttribute('data-category-id');
                    let newIndex = evt.newIndex;

                    htmx.ajax('POST', '/tasks/move', {
                        values: {
                            task_id: taskId,
                            category_id: toCatId,
                            index: newIndex
                        },
                        swap: 'none'
                    });
                }
            });
            el.sortableInitialized = true;
        }
    });

    // Initialize Sortable for Subtasks in Slideover AND Main Page Inline Lists
    // We want main page subtasks to be reorderable but NOT draggable outside their parent.
    // Slideover subtasks also need reordering.

    // Combining selector for both:
    // [id^=subtasks-list-] covers both if we name them consistently.
    // In index.html we used id="subtasks-list-{{.ID}}"
    // In details.html id="subtasks-list-{{.ID}}"
    // Wait, creating duplicate IDs if details slideover is open! 
    // Actually details uses `subtasks-list-{{.ID}}`. Index uses `subtasks-list-{{.ID}}`.
    // If slideover is open, we have valid HTML issue?
    // HTMX swaps innerHTML of slideover container. If we open task details, we might have collision if we don't be careful.
    // But `Sortable` inits on elements.

    // Let's target them all.
    document.querySelectorAll('[id^=subtasks-list-]').forEach(function (el) {
        if (!el.sortableInitialized) {
            new Sortable(el, {
                group: 'subtasks-' + el.id, // Unique group per list preventing cross-list dragging
                animation: 150,
                handle: '.subtask-handle',
                ghostClass: 'ghost',
                onEnd: function (evt) {
                    let taskId = el.id.replace('subtasks-list-', '');
                    let ids = [];
                    // Handle both .subtask (slideover) and .subtask-inline (main page)
                    el.querySelectorAll('[data-subtask-id]').forEach(function (item) {
                        ids.push(item.getAttribute('data-subtask-id'));
                    });

                    htmx.ajax('POST', '/subtasks/reorder', {
                        values: {
                            task_id: taskId,
                            id: ids
                        },
                        swap: 'none'
                    });
                }
            });
            el.sortableInitialized = true;
        }
    });
});

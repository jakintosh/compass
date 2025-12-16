document.addEventListener('htmx:load', function (evt) {
    // Initialize Sortable for Categories - drag from anywhere except interactive elements
    let categoriesList = document.getElementById('categories-list');
    if (categoriesList && !categoriesList.sortableInitialized) {
        new Sortable(categoriesList, {
            animation: 150,
            draggable: '.category',
            handle: '.drag-handle',
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

    // Initialize Sortable for Tasks within Categories - drag from anywhere except interactive elements  
    document.querySelectorAll('.tasks-list').forEach(function (el) {
        if (!el.sortableInitialized) {
            new Sortable(el, {
                group: 'tasks',
                animation: 150,
                draggable: '.task-item',
                handle: '.drag-handle',
                ghostClass: 'ghost',
                onMove: function (evt) {
                    return !evt.related.classList.contains('task-add-item');
                },
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

    // Initialize Sortable for Subtasks - drag from anywhere except interactive elements
    document.querySelectorAll('[id^=subtasks-list-]').forEach(function (el) {
        if (!el.sortableInitialized) {
            new Sortable(el, {
                group: 'subtasks-' + el.id,
                animation: 150,
                draggable: '.subtask',
                handle: '.drag-handle',
                ghostClass: 'ghost',
                onMove: function (evt) {
                    return !evt.related.classList.contains('subtask-add-item');
                },
                onEnd: function (evt) {
                    let taskId = el.id.replace('subtasks-list-', '');
                    let ids = [];
                    el.querySelectorAll('[data-id]').forEach(function (item) {
                        ids.push(item.getAttribute('data-id'));
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

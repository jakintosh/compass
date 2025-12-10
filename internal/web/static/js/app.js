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

    // Initialize Sortable for Subtasks in Slideover
    document.querySelectorAll('[id^=subtasks-list-]').forEach(function (el) {
        if (!el.sortableInitialized) {
            new Sortable(el, {
                animation: 150,
                handle: '.subtask-handle',
                ghostClass: 'ghost',
                onEnd: function (evt) {
                    let taskId = el.id.replace('subtasks-list-', '');
                    let ids = [];
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

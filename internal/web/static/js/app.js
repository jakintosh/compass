document.addEventListener("htmx:load", function (evt) {
  if (window._hyperscript && window._hyperscript.processNode) {
    const target = evt.detail && evt.detail.elt ? evt.detail.elt : evt.target;
    if (target && target !== document.body) {
      window._hyperscript.processNode(target);
    }
  }

  // Initialize Sortable for Categories
  let categoriesList = document.getElementById("categories-list");
  if (categoriesList && !categoriesList.sortableInitialized) {
    new Sortable(categoriesList, {
      animation: 150,
      draggable: ".category",
      handle: ".drag-handle",
      ghostClass: "ghost",
      onEnd: function () {
        let ids = this.toArray();
        htmx.ajax("POST", "/categories/reorder", {
          values: { id: ids },
          swap: "none",
        });
      },
    });
    categoriesList.sortableInitialized = true;
  }

  // Initialize Sortable for Tasks within Categories
  document.querySelectorAll(".tasks-list").forEach(function (el) {
    if (!el.sortableInitialized) {
      new Sortable(el, {
        animation: 150,
        draggable: ".task-item",
        handle: ".drag-handle",
        ghostClass: "ghost",
        onEnd: function () {
          let catId = el.getAttribute("data-category-id");
          let ids = [];
          el.querySelectorAll(".task-item[data-id]").forEach(function (item) {
            ids.push(item.getAttribute("data-id"));
          });

          htmx.ajax("POST", "/tasks/reorder", {
            values: {
              category_id: catId,
              id: ids,
            },
            swap: "none",
          });
        },
      });
      el.sortableInitialized = true;
    }
  });

  // Initialize Sortable for Subtasks
  document.querySelectorAll(".subtasks-list").forEach(function (el) {
    if (!el.sortableInitialized) {
      new Sortable(el, {
        group: "subtasks-" + el.id,
        animation: 150,
        draggable: ".subtask",
        handle: ".drag-handle",
        ghostClass: "ghost",
        onEnd: function (evt) {
          let taskId = el.id.replace("subtasks-list-", "");
          let ids = [];
          el.querySelectorAll("[data-id]").forEach(function (item) {
            ids.push(item.getAttribute("data-id"));
          });

          htmx.ajax("POST", "/subtasks/reorder", {
            values: {
              task_id: taskId,
              id: ids,
            },
            swap: "none",
          });
        },
      });
      el.sortableInitialized = true;
    }
  });
});

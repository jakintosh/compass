// Get CSRF token from meta tag if present
function getCsrfToken() {
  const meta = document.querySelector('meta[name="csrf-token"]');
  return meta ? meta.getAttribute("content") : "";
}

// Helper to add CSRF to values object
function withCsrf(values) {
  const csrf = getCsrfToken();
  if (csrf) {
    values.csrf = csrf;
  }
  return values;
}

function appURL(path) {
  const basePath = document.body ? document.body.dataset.basePath || "" : "";
  return basePath + path;
}

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
        htmx.ajax("POST", appURL("/categories/reorder"), {
          values: withCsrf({ id: ids }),
          swap: "none",
        });
      },
    });
    categoriesList.sortableInitialized = true;
  }

  // Initialize Sortable for Projects within Categories
  document.querySelectorAll(".projects-list").forEach(function (el) {
    if (!el.sortableInitialized) {
      new Sortable(el, {
        animation: 150,
        draggable: ".project-item",
        handle: ".drag-handle",
        ghostClass: "ghost",
        onEnd: function () {
          let catId = el.getAttribute("data-category-id");
          let ids = [];
          el.querySelectorAll(".project-item[data-id]").forEach(function (item) {
            ids.push(item.getAttribute("data-id"));
          });

          htmx.ajax("POST", appURL("/projects/reorder"), {
            values: withCsrf({
              category_id: catId,
              id: ids,
            }),
            swap: "none",
          });
        },
      });
      el.sortableInitialized = true;
    }
  });

  // Initialize Sortable for Tasks
  document.querySelectorAll(".tasks-list").forEach(function (el) {
    if (!el.sortableInitialized) {
      new Sortable(el, {
        group: "tasks-" + el.id,
        animation: 150,
        draggable: ".task",
        handle: ".drag-handle",
        ghostClass: "ghost",
        onEnd: function (evt) {
          let projectId = el.id.replace("tasks-list-", "");
          let ids = [];
          el.querySelectorAll("[data-id]").forEach(function (item) {
            ids.push(item.getAttribute("data-id"));
          });

          htmx.ajax("POST", appURL("/tasks/reorder"), {
            values: withCsrf({
              project_id: projectId,
              id: ids,
            }),
            swap: "none",
          });
        },
      });
      el.sortableInitialized = true;
    }
  });
});

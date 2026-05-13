(function () {
    function applyStateToElement(el) {
        const key = el.getAttribute("data-ui-key");
        const stateClass = el.getAttribute("data-ui-class");
        if (!key || !stateClass) {
            return;
        }
        const defaultValue =
            el.getAttribute("data-ui-default") === "true";
        const stored = localStorage.getItem(key);
        const shouldApply =
            stored === null ? defaultValue : stored === "true";
        el.classList.toggle(stateClass, shouldApply);
    }

    function scanNode(node) {
        if (!node) {
            return;
        }
        if (
            node.nodeType === 1 &&
            node.hasAttribute("data-ui-key") &&
            node.hasAttribute("data-ui-class")
        ) {
            applyStateToElement(node);
        }
        if (node.querySelectorAll) {
            node.querySelectorAll(
                "[data-ui-key][data-ui-class]",
            ).forEach(applyStateToElement);
        }
    }

    function applyStateToHTML(html) {
        if (!html) {
            return html;
        }
        const template = document.createElement("template");
        template.innerHTML = html;
        scanNode(template.content);
        return template.innerHTML;
    }

    const observer = new MutationObserver(function (mutations) {
        for (const mutation of mutations) {
            mutation.addedNodes.forEach(scanNode);
        }
    });

    observer.observe(document.documentElement, {
        childList: true,
        subtree: true,
    });

    function handleSwap(evt) {
        const detail = evt.detail;
        if (!detail || detail.shouldSwap === false) {
            return;
        }
        if (typeof detail.serverResponse !== "string") {
            return;
        }
        detail.serverResponse = applyStateToHTML(
            detail.serverResponse,
        );
    }

    document.addEventListener("htmx:beforeSwap", handleSwap);
    document.addEventListener("htmx:oobBeforeSwap", handleSwap);
})();

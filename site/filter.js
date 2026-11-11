const input = document.getElementById("filter");
const rows = [...document.querySelectorAll("#modules [data-search]")];
const links = [...document.querySelectorAll("[data-category]")];
let category = new URLSearchParams(location.search).get("category") ?? "";

function apply() {
    const terms = input.value.toLowerCase().split(/\s+/).filter(Boolean);
    for (const row of rows) {
        const listed = category === "" || row.dataset.categories.split(" ").includes(category);
        row.hidden = !listed || !terms.every((term) => row.dataset.search.includes(term));
    }
    for (const link of links) {
        link.toggleAttribute("aria-current", link.dataset.category === category);
    }
}

for (const link of links) {
    link.addEventListener("click", (event) => {
        event.preventDefault();
        category = link.dataset.category;
        const url = category === "" ? location.pathname : `?category=${category}`;
        history.replaceState(null, "", url);
        apply();
    });
}

input.addEventListener("input", apply);
apply();

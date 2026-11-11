const copied = 1500;

async function copy(button) {
    try {
        await navigator.clipboard.writeText(button.dataset.copy);
        button.textContent = "Copied";
    } catch {
        const command = button.previousElementSibling.querySelector("b");
        getSelection().selectAllChildren(command);
        button.textContent = "Selected";
    }
    setTimeout(() => {
        button.textContent = "Copy";
    }, copied);
}

for (const button of document.querySelectorAll(".copy")) {
    button.addEventListener("click", () => copy(button));
}

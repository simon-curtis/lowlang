const shown = 1500;

class CopyButton {
    #button;
    #status;
    #reset;

    constructor(button) {
        this.#button = button;
        this.#status = button.querySelector("[role=status]");
        button.addEventListener("click", () => this.copy());
    }

    async copy() {
        try {
            await navigator.clipboard.writeText(this.#button.dataset.copy);
            this.#show("copied", "Copied");
        } catch {
            const command = this.#button.previousElementSibling.querySelector("b");
            getSelection().selectAllChildren(command);
            this.#show("selected", "Selected");
        }
    }

    #show(state, message) {
        clearTimeout(this.#reset);
        this.#button.dataset.state = state;
        this.#status.textContent = message;
        this.#reset = setTimeout(() => {
            delete this.#button.dataset.state;
            this.#status.textContent = "";
        }, shown);
    }
}

for (const button of document.querySelectorAll(".copy")) {
    new CopyButton(button);
}

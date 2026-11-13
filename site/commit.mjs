import { execFile } from "node:child_process";
import { promisify } from "node:util";

const run = promisify(execFile);
const repository = new URL("../", import.meta.url);

export async function headCommit() {
    try {
        const { stdout } = await run("git", ["rev-parse", "HEAD"], { cwd: repository });
        return stdout.trim();
    } catch (error) {
        throw new Error(`git rev-parse HEAD: ${error.message}`);
    }
}

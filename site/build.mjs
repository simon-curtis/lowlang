import { docs } from "./docs.mjs";
import { packages } from "./packages.mjs";
import { Session } from "./lsp.mjs";
import { snippets } from "./snippets.mjs";

const session = new Session();
try {
    await session.start();
    const { pages, unchecked } = await docs(session);
    console.log(`docs: ${pages} pages`);
    for (const snippet of unchecked) {
        console.warn(`unchecked: ${snippet}`);
    }
    if (unchecked.length > 0) {
        throw new Error(`${unchecked.length} docs snippets fail to check`);
    }
    const modules = await packages(session.binary);
    console.log(`packages: ${modules} modules`);
    const blocks = await snippets(session);
    console.log(`snippets: ${blocks}`);
} catch (error) {
    console.error(`build: ${error.message}`);
    process.exitCode = 1;
} finally {
    await session.stop();
}

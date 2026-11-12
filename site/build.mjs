import { builtins, docs } from "./docs.mjs";
import { Library } from "./links.mjs";
import { packages, standard } from "./packages.mjs";
import { Session } from "./lsp.mjs";
import { snippets } from "./snippets.mjs";

const session = new Session();
try {
    await session.start();
    const modules = await standard(session.binary);
    const builtin = await builtins();
    const library = new Library(modules, builtin);
    const { pages, unchecked } = await docs(session, library);
    console.log(`docs: ${pages} pages`);
    for (const snippet of unchecked) {
        console.warn(`unchecked: ${snippet}`);
    }
    if (unchecked.length > 0) {
        throw new Error(`${unchecked.length} docs snippets fail to check`);
    }
    const count = await packages(modules, session, library);
    console.log(`packages: ${count} modules`);
    const blocks = await snippets(session, library);
    console.log(`snippets: ${blocks}`);
} catch (error) {
    console.error(`build: ${error.message}`);
    process.exitCode = 1;
} finally {
    await session.stop();
}

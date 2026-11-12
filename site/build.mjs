import { builtins, docs } from "./docs.mjs";
import { Library } from "./links.mjs";
import { packages, standard } from "./packages.mjs";
import { Session } from "./lsp.mjs";
import { pages } from "./pages.mjs";

const session = new Session();
try {
    await session.start();
    const modules = await standard(session.binary);
    const builtin = await builtins();
    const library = new Library(modules, builtin);
    const documented = await docs(session, library);
    console.log(`docs: ${documented} pages`);
    const count = await packages(modules, session, library);
    console.log(`packages: ${count} modules`);
    const blocks = await pages(session, library);
    console.log(`pages: ${blocks}`);
    session.verify();
} catch (error) {
    console.error(`build: ${error.message}`);
    process.exitCode = 1;
} finally {
    await session.stop();
}

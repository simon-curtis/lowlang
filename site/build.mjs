import { builtins, docs } from "./docs.mjs";
import { Library } from "./links.mjs";
import { packages, standard } from "./packages.mjs";
import { Session } from "./lsp.mjs";
import { fetchRoadmap } from "./roadmap.mjs";
import { SearchIndex } from "./search.mjs";
import { lexed } from "./source.mjs";
import { pages } from "./pages.mjs";

const session = new Session();
const search = new SearchIndex();
try {
    await session.start();
    const modules = await standard(session.binary);
    const design = await lexed("docs/design.md");
    const builtin = builtins(design);
    const library = new Library(modules, builtin);
    const roadmap = await fetchRoadmap();
    const documented = await docs(design, session, library, roadmap, search);
    console.log(`docs: ${documented} pages`);
    const count = await packages(modules, design, session, library, search);
    console.log(`packages: ${count} modules`);
    const blocks = await pages(session, library, search);
    console.log(`pages: ${blocks}`);
    const indexed = await search.write();
    console.log(`search: ${indexed}`);
    session.verify();
} catch (error) {
    console.error(`build: ${error.message}`);
    process.exitCode = 1;
} finally {
    await session.stop();
}

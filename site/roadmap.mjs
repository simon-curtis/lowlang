import { Marked } from "marked";
import { codespan } from "./highlight.mjs";
import { repository } from "./config.mjs";
import { escape } from "./markup.mjs";

const api = "https://api.github.com";
const slug = new URL(repository).pathname.slice(1);
const label = "roadmap";
const nextLink = /<([^>]+)>;\s*rel="next"/;
const marked = new Marked({ renderer: { codespan, html: ({ text }) => escape(text) } });

function headers() {
    const base = { Accept: "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28", "User-Agent": "low-site-build" };
    const token = process.env.GITHUB_TOKEN;
    return token ? { ...base, Authorization: `Bearer ${token}` } : base;
}

async function fetchPage(url) {
    let response;
    try {
        response = await fetch(url, { headers: headers() });
    } catch (error) {
        throw new Error(`roadmap: cannot reach ${url}: ${error.message}`);
    }
    if (!response.ok) {
        throw new Error(`roadmap: ${url} answered ${response.status} ${response.statusText}`);
    }
    try {
        return { items: await response.json(), next: nextLink.exec(response.headers.get("link") ?? "")?.[1] };
    } catch (error) {
        throw new Error(`roadmap: ${url} returned unreadable JSON: ${error.message}`);
    }
}

async function fetchAll(url) {
    const items = [];
    for (let next = url; next; ) {
        const page = await fetchPage(next);
        items.push(...page.items);
        next = page.next;
    }
    return items;
}

async function describe(issue) {
    const children = issue.sub_issues_summary?.total > 0 ? await fetchAll(`${api}/repos/${slug}/issues/${issue.number}/sub_issues?per_page=100`) : [];
    const items = [];
    for (const child of children) {
        items.push(await describe(child));
    }
    return { number: issue.number, title: issue.title, body: issue.body ?? "", url: issue.html_url, done: issue.state === "closed", items };
}

export async function fetchRoadmap() {
    const issues = await fetchAll(`${api}/repos/${slug}/issues?labels=${label}&state=all&sort=created&direction=asc&per_page=100`);
    const roots = issues.filter((issue) => !issue.pull_request);
    if (roots.length === 0) {
        throw new Error(`roadmap: ${slug} has no issues labelled ${label}`);
    }
    const items = [];
    for (const issue of roots) {
        items.push(await describe(issue));
    }
    return items;
}

function entry(item) {
    const badge = item.done ? ' <span class="font-mono text-xs font-semibold uppercase">Done</span>' : "";
    const body = item.body.trim() ? `\n<p>${marked.parseInline(item.body.trim())}</p>` : "";
    const nested = item.items.length > 0 ? `\n${list(item.items)}` : "";
    return `<li><a href="${escape(item.url)}">${escape(item.title)}</a>${badge}${body}${nested}</li>`;
}

function list(items) {
    return `<ol>\n${items.map(entry).join("\n")}\n</ol>`;
}

export function renderRoadmap(items) {
    const intro = `<p>The roadmap is the <a href="${repository}/issues?q=label%3A${label}">issues labelled <code>${label}</code></a> on GitHub, with each item's sub-issues nested beneath it.</p>`;
    return `${intro}\n${list(items)}\n`;
}

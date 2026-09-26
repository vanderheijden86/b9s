#!/usr/bin/env node
// A stand-in for the bd CLI. It applies the handful of writes b9s web runs
// to .beads/issues.jsonl in the working directory, so the browser tests
// exercise the real server, write path and file watcher without touching
// any Beads database. Every call is appended to .beads/bd-calls.log.

import fs from "node:fs";
import path from "node:path";

const args = process.argv.slice(2);
const dir = path.join(process.cwd(), ".beads");
const file = path.join(dir, "issues.jsonl");
fs.appendFileSync(path.join(dir, "bd-calls.log"), JSON.stringify(args) + "\n");

if (process.env.FAKE_BD_FAIL && args[0] === process.env.FAKE_BD_FAIL) {
  process.stderr.write("fake bd: refused by FAKE_BD_FAIL\n");
  process.exit(1);
}

const issues = fs.readFileSync(file, "utf8").split("\n").filter(Boolean).map(l => JSON.parse(l));
const byId = new Map(issues.map(i => [i.id, i]));
const now = new Date().toISOString();

const flags = {};
const ids = [];
let rest = [];
for (let n = 1; n < args.length; n++) {
  const a = args[n];
  if (a === "--") { rest = args.slice(n + 1); break; }
  const m = a.match(/^--([\w-]+)(?:=(.*))?$/s);
  if (m) flags[m[1]] = m[2] ?? "true"; else ids.push(a);
}

function need(id) {
  const i = byId.get(id);
  if (!i) { process.stderr.write(`fake bd: no issue ${id}\n`); process.exit(1); }
  return i;
}

function setStatus(i, status) {
  i.status = status;
  i.updated_at = now;
  if (status === "closed") i.closed_at = now; else delete i.closed_at;
}

switch (args[0]) {
  case "update":
    for (const id of ids) {
      const i = need(id);
      for (const [k, v] of Object.entries(flags)) {
        if (k === "status") setStatus(i, v);
        else if (k === "priority") i.priority = Number(v);
        else if (k === "type") i.issue_type = v;
        else if (k === "add-label") i.labels = [...new Set([...(i.labels || []), ...v.split(",")])];
        else if (k === "remove-label") i.labels = (i.labels || []).filter(l => !v.split(",").includes(l));
        else if (k === "acceptance") i.acceptance_criteria = v;
        else if (k === "parent") {
          i.dependencies = (i.dependencies || []).filter(d => d.type !== "parent-child");
          if (v) i.dependencies.push({ issue_id: id, depends_on_id: v, type: "parent-child", created_at: now, created_by: "fake" });
        } else i[k] = v;
      }
      i.updated_at = now;
    }
    break;
  case "close":
    for (const id of ids) { const i = need(id); setStatus(i, "closed"); if (flags.reason) i.close_reason = flags.reason; }
    break;
  case "delete":
    for (const id of ids) {
      need(id);
      byId.delete(id);
    }
    for (const i of byId.values()) i.dependencies = (i.dependencies || []).filter(d => !ids.includes(d.depends_on_id));
    break;
  case "defer":
    for (const id of ids) { const i = need(id); setStatus(i, "deferred"); if (flags.until) i.defer_until = new Date(flags.until).toISOString(); }
    break;
  case "comments": {
    const id = ids[1];
    const i = need(id);
    i.comments = [...(i.comments || []), { id: String(Date.now()), issue_id: id, author: "fake-actor", text: rest.join(" "), created_at: now }];
    i.updated_at = now;
    break;
  }
  case "create": {
    const prefix = (issues[0]?.id || "t-0").split("-")[0];
    let n = byId.size + 1;
    while (byId.has(`${prefix}-n${n}`)) n++;
    const id = `${prefix}-n${n}`;
    const i = {
      id, title: flags.title || "", description: flags.description || "", status: "open",
      priority: Number(flags.priority ?? 2), issue_type: flags.type || "task", assignee: flags.assignee || "",
      created_at: now, updated_at: now, created_by: "fake-actor",
      labels: flags.labels ? flags.labels.split(",") : [],
      dependencies: flags.parent ? [{ issue_id: id, depends_on_id: flags.parent, type: "parent-child", created_at: now, created_by: "fake" }] : [],
    };
    byId.set(id, i);
    process.stdout.write(`Created issue: ${id}\n`);
    break;
  }
  default:
    process.stderr.write(`fake bd: unsupported ${args[0]}\n`);
    process.exit(2);
}

const tmp = file + ".tmp";
fs.writeFileSync(tmp, [...byId.values()].map(i => JSON.stringify(i)).join("\n") + "\n");
fs.renameSync(tmp, file);

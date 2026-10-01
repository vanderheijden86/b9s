import { expect, open, row, test } from "./harness";

const proof = `\`\`\`mermaid
sequenceDiagram
    participant User
    participant b9s
    User->>b9s: Open detail
    b9s-->>User: Render diagram
\`\`\``;

const largeSequence = `\`\`\`mermaid
sequenceDiagram
    autonumber
    actor Operator
    participant Browser as React client
    participant API as Express dashboard API
    participant DB as PostgreSQL
    participant Function as Azure Functions

    Note over Browser,API: On App Service, Easy Auth signs the operator in with Entra ID<br/>and adds the principal header before Express sees the request
    Browser->>API: GET /api/me
    API->>API: admin may act, rwssync.viewer may GET and post<br/>the developer routes, anyone else gets 403
    API-->>Browser: name, roles, canWrite
    Browser->>Browser: without canWrite, hide retry and disable the<br/>endpoint tester's sending endpoints
    Browser->>API: GET /api/summary or /api/tasks
    API->>DB: Prisma query, raw SQL names schema.table
    Note over API,DB: ACC and PRD pool through PgBouncer in transaction mode,<br/>where a pooled connection can lack the app search_path
    alt Query succeeds
        DB-->>API: Operational data
        API->>API: externalIdWarning from state, rwsRecordNumber<br/>and the stored push's ExternalId
        API-->>Browser: JSON
        Note over Browser: A matching code shows the RWS number once.<br/>ExternalId ontbreekt or ExternalId wijkt af only when an operator must act
    else Query fails
        DB-->>API: Error
        API-->>Browser: 500 with the error message
        Note over API: asyncRoute passes the rejection to the error middleware,<br/>so the process keeps serving other requests
    end
    Browser->>API: GET /api/events
    API-->>Browser: Open SSE stream
    loop Every five seconds
        API->>DB: Query latest task
        alt Query succeeds
            DB-->>API: Latest task state
            API-->>Browser: update event
            Browser->>Browser: Refresh visible queries
        else Query fails
            API->>API: log it and skip this update, stream stays open
        end
    end
    Note over API,DB: GET /api/tasks answers one row per environment and sourceJobId,<br/>the latest sync with its sync count. Filters judge that latest sync
    Note over API,DB: GET /api/summary counts the same job rows, so state, failure<br/>and gate counts judge each job by its latest sync only
    opt Operator expands a job Ultimo pushed more than once
        Browser->>API: GET /api/tasks/id/syncs
        API->>DB: every task with the same environment and sourceJobId
        API-->>Browser: syncs, newest first
        Browser->>Browser: list the earlier syncs under the row
    end
    opt Operator opens a task from a filtered Jobs list
        Browser->>Browser: remember the list query in sessionStorage
        Browser->>API: GET /api/tasks/id and GET /api/tasks/id/neighbours?filters
        API->>DB: the task, plus the job rows directly above and below it
        Note over API,DB: Same job rows, filters and createdAt then id order<br/>as GET /api/tasks, so Vorige and Volgende<br/>step through the list the operator left
        API-->>Browser: task, previous, next
    end
    opt An admin retries a failed task
        Browser->>API: POST /api/tasks/id/retry
        API->>API: 403 unless the operator holds admin
        API->>Function: POST /sync/tasks/id/retry with function key
        Function-->>API: 202 or conflict
        API-->>Browser: Result
    end
\`\`\``;

test.use({ fixture: { issues: [
  { id: "mermaid-proof", title: "Sequence proof", description: proof },
  { id: "mermaid-large", title: "Large sequence", description: largeSequence },
  { id: "mermaid-flow", title: "Flowchart", description: "```mermaid\ngraph LR\nA[Start] --> B[Done]\n```" },
  { id: "mermaid-invalid", title: "Invalid diagram", description: "```mermaid\nsequenceDiagram\nAlice->>Bob\n```" },
  { id: "mermaid-untrusted", title: "Untrusted label", description: "```mermaid\nsequenceDiagram\nparticipant Alice as <img src=x onerror=alert(1)>\nAlice->>Alice: Hello\n```" },
] } });

test("web detail renders mirrored sequence participants", async ({ page, project }) => {
  await open(page, project);
  await row(page, "mermaid-proof").click();
  const diagram = page.locator('.detail .mermaid-diagram svg');
  await expect(diagram).toBeVisible();
  await expect(diagram.locator("text", { hasText: "User" })).toHaveCount(2);
  await expect(diagram.locator("text", { hasText: "b9s" })).toHaveCount(2);
  await expect(page.locator(".detail .md pre code")).toHaveCount(0);
  await page.locator('.detail [data-act="edit"]').click();
  await expect(page.locator("#fDesc")).toHaveValue(proof);
});

test("web detail renders a flowchart", async ({ page, project }) => {
  await open(page, project);
  await row(page, "mermaid-flow").click();
  const diagram = page.locator('.detail .mermaid-diagram svg');
  await expect(diagram).toBeVisible();
  await expect(diagram).toContainText("Start");
  await expect(diagram).toContainText("Done");
});

test("invalid Mermaid stays as escaped source", async ({ page, project }) => {
  await open(page, project);
  await row(page, "mermaid-invalid").click();
  await expect(page.locator(".detail .mermaid-diagram")).toHaveAttribute("data-render-state", "error");
  await expect(page.locator(".detail .mermaid-diagram pre code")).toContainText("Alice->>Bob");
  await expect(page.locator(".detail .mermaid-diagram svg")).toHaveCount(0);
});

test("untrusted labels cannot create HTML elements", async ({ page, project }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await open(page, project);
  await row(page, "mermaid-untrusted").click();
  await expect(page.locator(".detail .mermaid-diagram svg")).toBeVisible();
  await expect(page.locator(".detail .mermaid-diagram img, .detail .mermaid-diagram script")).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("the supplied large sequence renders within five seconds", async ({ page, project }) => {
  const errors: string[] = [];
  const requests: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  page.on("request", request => { if (request.url().endsWith("/mermaid.min.js")) requests.push(request.url()); });
  await open(page, project);
  expect(requests).toHaveLength(0);
  const started = Date.now();
  await row(page, "mermaid-large").click();
  const diagram = page.locator('.detail .mermaid-diagram svg');
  await expect(diagram).toBeVisible({ timeout: 5000 });
  const elapsed = Date.now() - started;
  expect(elapsed).toBeLessThan(5000);
  await expect(diagram).toContainText("Azure Functions");
  await expect(diagram).toContainText("Vorige and Volgende");
  const dimensions = await diagram.evaluate(svg => ({
    content: svg.parentElement!.scrollWidth,
    viewport: svg.parentElement!.clientWidth,
  }));
  expect(dimensions.content).toBeGreaterThan(dimensions.viewport);
  expect(requests).toHaveLength(1);
  expect(errors).toEqual([]);
  test.info().annotations.push({ type: "perf", description: `cold large sequence: ${elapsed} ms` });
});

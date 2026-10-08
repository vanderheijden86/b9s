import { test, expect, open } from "./harness";
const graph={version:1,available:true,nodes:[
 {id:"m-a",kind:"memory",title:"Read through the matching CLI",status:"",type:"s/types/preview-memory-v2",record_version:"v1",body:"## Rationale\nUse **one graph**."},
 {id:"m-b",kind:"memory",title:"Keep preview data isolated",status:"",type:"s/types/preview-memory-v2",record_version:"v2",body:"---\nstatus: superseded\n---\nLiteral body"},
 {id:"t-1",kind:"issue",title:"Build the Memory browser",status:"open",type:"s/types/preview-issue-v2",record_version:"v3",body:"Work description."},
 {id:"t-2",kind:"issue",title:"Serve the browser UI",status:"open",type:"s/types/preview-issue-v2",record_version:"v4",body:"Other work."},
 {id:"t-3",kind:"issue",title:"Unlinked work",status:"closed",type:"s/types/preview-issue-v2",record_version:"v5",body:"Done."},
],edges:[
 {id:"f",source:"t-1",target:"m-a",kind:"follows",type:"s/types/example-follows",note:"Implementation policy"},
 {id:"c",source:"t-1",target:"m-b",kind:"cites",type:"s/types/example-cites",note:"Read in isolation"},
 {id:"r",source:"m-a",target:"m-b",kind:"related",type:"s/types/preview-related-v2",note:"Context"},
 {id:"b",source:"t-2",target:"t-1",kind:"blocks",type:"s/types/preview-blocks-v1",note:""},
]};
test.beforeEach(async({page})=>{await page.route("**/api/memory-graph",route=>route.fulfill({json:graph}));});
async function graphView(page: import('@playwright/test').Page,project:{url:string}){await open(page,project as Parameters<typeof open>[1]);await page.locator('[data-view="memory"]').click();await expect(page.locator('#vMemory [data-node]')).toHaveCount(5);}
test("native graph uses full card titles and four stored Link Types",async({page,project})=>{
 await graphView(page,project);
 await expect(page.locator('[data-node="m-a"] rect')).toHaveCount(1);
 await expect(page.locator('[data-node="m-a"]')).toContainText("Read through the");
 for(const kind of ["follows","cites","related","blocks"])await expect(page.locator(`[data-edge-kind="${kind}"]`)).toHaveCount(1);
 await expect(page.locator('#vMemory')).not.toContainText("Problems only");
 await expect(page.locator('#vMemory .memoryLegend')).not.toContainText("superseded");
 await expect(page.locator('#vMemory .memoryLegend')).toContainText("depends on");
});
test("selection and filters keep card coordinates stable",async({page,project})=>{
 await graphView(page,project);const before=await page.locator('[data-node="m-a"]').getAttribute('transform');
 await page.locator('#memoryFit').click();
 await page.locator('[data-node="m-a"] .memoryBead').click();await expect(page.locator('#memoryDetail .md b')).toHaveText('one graph');
 await page.locator('#memoryType').selectOption('cites');await expect(page.locator('[data-edge-kind]')).toHaveCount(1);
 await expect(page.locator('[data-node="m-a"]')).toHaveAttribute('transform',before!);
});
test("Link selection exposes exact Type and dependency direction",async({page,project})=>{
 await graphView(page,project);await page.locator('#memoryType').selectOption('blocks');
 await page.locator('[data-edge="b"] .memoryEdgeLabel').click();
 await expect(page.locator('#memoryDetail')).toContainText('preview-blocks-v1');
 await expect(page.locator('#memoryDetail')).toContainText('depends on');
 await expect(page.locator('#memoryDetail')).toContainText('Serve the browser UI');
});
test("keyboard selection highlights only neighbouring records",async({page,project})=>{
 await graphView(page,project);await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 await expect(page.locator('[data-node="t-3"]')).toHaveClass(/memoryDim/);
 await expect(page.locator('#memoryDetail')).toContainText('Work description.');
});
test("record shapes distinguish Issues from Memories",async({page,project})=>{
 await graphView(page,project);
 await expect(page.locator('[data-node="t-1"] circle.memoryCard')).toHaveCount(1);
 await expect(page.locator('[data-node="t-1"] rect.memoryCard')).toHaveCount(0);
 await expect(page.locator('[data-node="m-a"] rect.memoryCard')).toHaveAttribute('rx','13');
 const edge = await page.locator('[data-edge="f"] > path').getAttribute('d');
 const transform = await page.locator('[data-node="t-1"]').getAttribute('transform');
 const [x,y] = transform!.match(/-?[\d.]+/g)!.map(Number);
 const [sx,sy] = edge!.match(/-?[\d.]+/g)!.map(Number);
 expect(Math.hypot(sx-x-125,sy-y-51)).toBeCloseTo(51, 3);
});
test("unrelated records remain readable during selection",async({page,project})=>{
 await graphView(page,project);
 await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 const other=page.locator('[data-node="t-3"]');
 await expect(other).toHaveClass(/memoryDim/);
 await expect.poll(()=>other.evaluate(el=>Number(getComputedStyle(el).opacity))).toBeGreaterThanOrEqual(.7);
 await expect(page.locator('[data-node="t-1"]')).toHaveCSS('opacity','1');
});
test("fade slider sets the opacity of records outside the selection",async({page,project})=>{
 await graphView(page,project);
 const fade=page.locator('#memoryFade');
 await expect(fade).toHaveValue('75');
 await expect(page.locator('label:has(#memoryFade)')).toContainText('Unselected opacity 75%');
 await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 await fade.fill('20');
 await expect(page.locator('label:has(#memoryFade)')).toContainText('Unselected opacity 20%');
 const other=page.locator('[data-node="t-3"]');
 await expect.poll(()=>other.evaluate(el=>Number(getComputedStyle(el).opacity))).toBeCloseTo(.2,2);
 await expect.poll(()=>page.locator('[data-edge="r"]').evaluate(el=>Number(getComputedStyle(el).opacity))).toBeCloseTo(.12,2);
 await expect(page.locator('[data-node="t-1"]')).toHaveCSS('opacity','1');
 await page.locator('#memoryType').selectOption('cites');
 await expect(page.locator('#memoryFade')).toHaveValue('20');
 await expect.poll(()=>page.locator('[data-node="t-3"]').evaluate(el=>Number(getComputedStyle(el).opacity))).toBeCloseTo(.2,2);
});
test("Memory detail uses the tree detail layout",async({page,project})=>{
 await graphView(page,project);await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 const d=page.locator('#detailHost .detail#memoryDetail');
 await expect(d.locator('.dtop .id')).toHaveText('t-1');
 await expect(d.locator('.dtitle')).toHaveText('Build the Memory browser');
 await expect(d.locator('.pills')).toContainText('issue');
 await expect(d.locator('.pills')).toContainText('open');
 await expect(d.locator('.meta')).toContainText('v3');
 await expect(d.locator('.dsec').first()).toContainText('Links · 3');
 await expect(d.locator('.rel')).toHaveCount(3);
 await expect(d.locator('.rel .kn')).toHaveText(['1','2','3']);
 await expect(d.locator('.rel').first()).toContainText('Read through the matching CLI');
 await expect(d.locator('.dsec',{hasText:'Description'})).toHaveCount(1);
 await expect(d.locator('.md')).toContainText('Work description.');
 await expect(page.locator('#app')).toHaveClass(/hasdetail/);
 await d.locator('.rel').nth(1).click();
 await expect(d.locator('.dtitle')).toHaveText('Keep preview data isolated');
 await expect(d.locator('.back')).toContainText('t-1');
});
test("Memory detail keys match the tree detail",async({page,project})=>{
 await graphView(page,project);await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 const d=page.locator('#memoryDetail');
 await page.keyboard.press('1');
 await expect(d.locator('.dtitle')).toHaveText('Read through the matching CLI');
 await expect(page.locator('[data-node="m-a"]')).toHaveAttribute('aria-pressed','true');
 await page.keyboard.press('Backspace');
 await expect(d.locator('.dtitle')).toHaveText('Build the Memory browser');
 await page.keyboard.press('n');
 await expect(d.locator('.dtitle')).toHaveText('Serve the browser UI');
 await page.keyboard.press('p');
 await expect(d.locator('.dtitle')).toHaveText('Build the Memory browser');
 await page.keyboard.press('\\');
 await expect(d).toHaveClass(/full/);
 await page.keyboard.press('\\');
 await expect(d).not.toHaveClass(/full/);
 await page.keyboard.press('d');
 await expect(d).toHaveCount(0);
 await page.locator('[data-node="t-1"]').focus();await page.keyboard.press('Enter');
 await page.keyboard.press('Escape');
 await expect(d).toHaveCount(0);
 await expect(page.locator('#vMemory')).toBeVisible();
});
test("zoom survives node selection",async({page,project})=>{
 await graphView(page,project);await page.locator('#memoryZoomIn').click();const before=await page.locator('.memoryViewport').getAttribute('transform');
 await page.locator('[data-node="t-1"]').dispatchEvent('click');await expect(page.locator('.memoryViewport')).toHaveAttribute('transform',before!);
});
test("graph request is reused between views",async({page,project})=>{
 let calls=0;await page.route('**/api/memory-graph',r=>{calls++;return r.fulfill({json:graph});});await graphView(page,project);
 await page.locator('[data-view="tree"]').click();await page.locator('[data-view="memory"]').click();await expect(page.locator('[data-node]')).toHaveCount(5);expect(calls).toBe(1);
});
test("unavailable graph explains why in one line",async({page,project})=>{
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{version:1,available:false,reason:'bd at /usr/bin/bd has no Memory Beads support',nodes:[],edges:[]}}));await open(page,project);await page.locator('[data-view="memory"]').click();await expect(page.locator('#vMemory')).toContainText('Memories unavailable: bd at /usr/bin/bd has no Memory Beads support');
});
test("failed graph request can be retried",async({page,project})=>{
 let calls=0;await page.route('**/api/memory-graph',r=>r.fulfill(++calls===1?{status:503,json:{error:'Unavailable'}}:{json:graph}));await open(page,project);await page.locator('[data-view="memory"]').click();await expect(page.locator('#vMemory [role="alert"]')).toContainText('Unavailable');await page.locator('#memoryRetry').click();await expect(page.locator('[data-node]')).toHaveCount(5);
});
// A finished graph read is a new version (ADR 0051), so a graph newer than the
// snapshot fetches the snapshot before it is drawn.
test("graph newer than the snapshot refreshes the snapshot first",async({page,project})=>{
 let calls=0;await page.route('**/api/snapshot',async r=>{const response=await r.fetch();const snap=await response.json();await r.fulfill({json:{...snap,version:++calls>1?2:1}})});
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{...graph,version:2}}));await open(page,project);await page.locator('[data-view="memory"]').click();await expect(page.locator('[data-node]')).toHaveCount(5);expect(calls).toBe(2);
});
test("empty graph remains navigable",async({page,project})=>{
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{version:1,available:true,nodes:[],edges:[]}}));await open(page,project);await page.locator('[data-view="memory"]').click();await expect(page.locator('#vMemory')).toContainText('No records');await page.locator('[data-view="tree"]').click();await expect(page.locator('#vTree')).toBeVisible();
});
test("graph hash opens directly",async({page,project})=>{await page.goto(project.url+'#/memory');await expect(page.locator('[data-node]')).toHaveCount(5)});

// The server reads the graph on the first request (ADR 0051), so the view
// polls it and shows how far the read has come.
test("Memory view shows a progress bar until the graph is read",async({page,project})=>{
 let ready=false;
 await page.route("**/api/memory-graph",route=>route.fulfill({json:ready?graph:{version:1,available:false,loading:true,done:1,total:4,nodes:[],edges:[]}}));
 await open(page,project);await page.locator('[data-view="memory"]').click();
 const bar=page.locator('#vMemory [role="progressbar"]');
 await expect(bar).toHaveAttribute('aria-valuenow','1');await expect(bar).toHaveAttribute('aria-valuemax','4');
 await expect(page.locator('#vMemory')).toContainText('Reading Memory Links 1/4');
 ready=true;
 await expect(page.locator('#vMemory [data-node]')).toHaveCount(5);
});
test("Memory view names the inventory read before it counts Links",async({page,project})=>{
 await page.route("**/api/memory-graph",route=>route.fulfill({json:{version:1,available:false,loading:true,done:0,total:0,nodes:[],edges:[]}}));
 await open(page,project);await page.locator('[data-view="memory"]').click();
 await expect(page.locator('#vMemory [role="progressbar"]')).not.toHaveAttribute('aria-valuenow',/./);
 await expect(page.locator('#vMemory')).toContainText('Reading the Memory inventory');
});
test("Issue detail says its Memory Links are loading",async({page,project})=>{
 await page.route('**/api/issue?*',async route=>{
  const response=await route.fetch();const issue=await response.json();
  await route.fulfill({json:{...issue,memory_available:false,memory_loading:true,memory_links:[]}});
 });
 await open(page,project);
 await page.locator('#treeRows .row[data-id="t-1"]').click();
 await expect(page.locator('#detailHost')).toContainText('Loading Memory Links');
});

test("regular Issue detail shows incoming and outgoing Memory Links", async ({page,project}) => {
 await page.route('**/api/issue?*', async route => {
  const response = await route.fetch(); const issue = await response.json();
  await route.fulfill({json:{...issue,memory_available:true,memory_links:[
   {link:graph.edges[0],memory:graph.nodes[0],outgoing:true},
   {link:{...graph.edges[2],source:'m-b',target:'t-1'},memory:graph.nodes[1],outgoing:false},
  ]}});
 });
 await open(page,project);
 await page.locator('#treeRows .row[data-id="t-1"]').click();
 await expect(page.locator('#detailHost')).toContainText('Memory Links');
 await expect(page.locator('#detailHost')).toContainText('This Issue → follows → Memory');
 await expect(page.locator('#detailHost')).toContainText('Memory → related → this Issue');
 await page.locator('#detailHost .issueMemoryLink summary').first().click();
 await expect(page.locator('#detailHost .issueMemoryLink .md b').first()).toHaveText('one graph');
});

test("record picker brings an offscreen card into view at readable size", async ({page,project}) => {
 await graphView(page,project);
 await page.setViewportSize({width:412,height:915});
 await page.locator('#memoryRecord').selectOption('m-a');
 const box=await page.locator('[data-node="m-a"] rect').boundingBox();
 expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x+box!.width).toBeLessThanOrEqual(412);
 expect(box!.width).toBeGreaterThanOrEqual(240);
 await expect(page.locator('#memoryDetail .dtitle')).toHaveText('Read through the matching CLI');
});

test("unrecognized stored Types stay visible without invented semantics",async({page,project})=>{
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{...graph,nodes:[...graph.nodes,{id:'unknown',kind:'other',type:'s/types/custom',title:'Unrecognized record',status:'',body:'',record_version:'v1'}]}}));
 await open(page,project);await page.locator('[data-view="memory"]').click();
 await expect(page.locator('[data-node="unknown"]')).toContainText('Unrecognized record');
});

test("Memory detail renders a Mermaid body as a diagram",async({page,project})=>{
 const body="## Decision\n```mermaid\nsequenceDiagram\n    participant Phone\n    participant Web\n    Phone->>Web: GET /pair\n```";
 await page.route("**/api/memory-graph",route=>route.fulfill({json:{...graph,nodes:graph.nodes.map(n=>n.id==="m-a"?{...n,body}:n)}}));
 await graphView(page,project);await page.locator('[data-node="m-a"]').click();
 await expect(page.locator('#memoryDetail .mermaid-diagram svg')).toBeVisible();
 await expect(page.locator('#memoryDetail .mermaid-diagram svg text',{hasText:"Phone"})).toHaveCount(2);
 await expect(page.locator('#memoryDetail .md pre code')).toHaveCount(0);
});
test("hard-wrapped Memory body reads as one paragraph",async({page,project})=>{
 const body="**b9s web requires pairing\non every address.** The link\nworks until replaced.\n\nSecond paragraph.\n\n- Nothing but the secret\n  is stored.\n- Second item";
 await page.route("**/api/memory-graph",route=>route.fulfill({json:{...graph,nodes:graph.nodes.map(n=>n.id==="m-a"?{...n,body}:n)}}));
 await graphView(page,project);await page.locator('[data-node="m-a"]').click();
 await expect(page.locator('#memoryDetail .md p')).toHaveCount(2);
 await expect(page.locator('#memoryDetail .md b')).toHaveText('b9s web requires pairing on every address.');
 await expect(page.locator('#memoryDetail .md li')).toHaveText(['Nothing but the secret is stored.','Second item']);
});
test("line buttons set Link thickness within bounds",async({page,project})=>{
 await graphView(page,project);
 const path=page.locator('[data-edge="f"] > path'),out=page.locator('#memoryLine');
 await expect(out).toHaveText('2px');
 await expect(path).toHaveCSS('stroke-width','2px');
 await page.locator('#memoryLineUp').click();
 await expect(out).toHaveText('3px');await expect(path).toHaveCSS('stroke-width','3px');
 for(let i=0;i<2;i++)await page.locator("#memoryLineDown").click();
 await expect(out).toHaveText('1px');await expect(path).toHaveCSS('stroke-width','1px');
 await expect(page.locator('#memoryLineDown')).toBeDisabled();
});
test("Link kinds use UML connectors",async({page,project})=>{
 await graphView(page,project);
 const p=(id:string)=>page.locator(`[data-edge="${id}"] > path`);
 await expect(page.locator('[data-edge="f"]')).toHaveAttribute('data-uml','realization');
 await expect(p('f')).toHaveAttribute('stroke-dasharray',/\d/);
 await expect(p('f')).toHaveAttribute('marker-end','url(#memory-head-hollow-follows)');
 await expect(page.locator('[data-edge="b"]')).toHaveAttribute('data-uml','dependency');
 await expect(p('b')).toHaveAttribute('stroke-dasharray',/\d/);
 await expect(p('b')).toHaveAttribute('marker-end','url(#memory-head-open-blocks)');
 await expect(page.locator('[data-edge="c"]')).toHaveAttribute('data-uml','directed association');
 await expect(p('c')).not.toHaveAttribute('stroke-dasharray',/./);
 await expect(p('c')).toHaveAttribute('marker-end','url(#memory-head-open-cites)');
 await expect(page.locator('[data-edge="r"]')).toHaveAttribute('data-uml','association');
 await expect(p('r')).not.toHaveAttribute('stroke-dasharray',/./);
 await expect(p('r')).not.toHaveAttribute('marker-end',/./);
 await expect(page.locator('.memoryLegend svg')).toHaveCount(4);
 await expect(page.locator('.memoryLegend')).toContainText('realization');
});
const zoomLevel=async(page:import('@playwright/test').Page)=>Number((await page.locator('.memoryCanvas #memoryZoomLevel').textContent())!.replace('%',''));
test("canvas zoom buttons show and change the zoom level",async({page,project})=>{
 await graphView(page,project);
 await expect(page.locator('.memoryCanvas #memoryZoomIn')).toBeVisible();
 await page.locator('.memoryCanvas').scrollIntoViewIfNeeded();await expect(page.locator('.memoryCanvas #memoryZoomIn')).toBeInViewport();
 const start=await zoomLevel(page);
 await page.locator('.memoryCanvas #memoryZoomIn').click();
 await expect.poll(()=>zoomLevel(page)).toBeGreaterThan(start);
 await page.locator('.memoryCanvas #memoryZoomOut').click();await page.locator('.memoryCanvas #memoryZoomOut').click();
 await expect.poll(()=>zoomLevel(page)).toBeLessThan(start);
});
test("keys zoom the Memory graph and f fits it",async({page,project})=>{
 await graphView(page,project);const start=await zoomLevel(page);
 await page.keyboard.press('+');await expect.poll(()=>zoomLevel(page)).toBeGreaterThan(start);
 await page.keyboard.press('-');await page.keyboard.press('-');await expect.poll(()=>zoomLevel(page)).toBeLessThan(start);
 await page.keyboard.press('f');await expect.poll(()=>zoomLevel(page)).toBe(start);
});
test("wheel zooms toward the pointer",async({page,project,isMobile})=>{
 test.skip(isMobile,'phones have no wheel');
 await graphView(page,project);const start=await zoomLevel(page);
 const card=page.locator('[data-node="t-1"] circle');const b=(await card.boundingBox())!;
 await page.mouse.move(b.x+b.width/2,b.y+b.height/2);await page.mouse.wheel(0,-300);
 await expect.poll(()=>zoomLevel(page)).toBeGreaterThan(start);
 const a=(await card.boundingBox())!;
 expect(Math.abs(a.x+a.width/2-(b.x+b.width/2))).toBeLessThan(4);
});
test("two-finger pinch zooms the Memory graph",async({page,project})=>{
 await graphView(page,project);const start=await zoomLevel(page);
 await page.locator('.memoryCanvas svg').evaluate(svg=>{
  const r=svg.getBoundingClientRect(),y=r.top+r.height/2,ev=(type:string,id:number,x:number)=>svg.dispatchEvent(new PointerEvent(type,{pointerId:id,clientX:x,clientY:y,bubbles:true,pointerType:'touch',isPrimary:id===1}));
  ev('pointerdown',1,r.left+100);ev('pointerdown',2,r.left+200);ev('pointermove',2,r.left+300);ev('pointerup',2,r.left+300);ev('pointerup',1,r.left+100);
 });
 await expect.poll(()=>zoomLevel(page)).toBeGreaterThan(start*1.5);
});
test("long Memory details can close",async({page,project})=>{
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{...graph,nodes:graph.nodes.map(n=>n.id==='m-a'?{...n,body:'Long paragraph.\n\n'.repeat(80)}:n)}}));
 await graphView(page,project);await page.locator('#memoryRecord').selectOption('m-a');
 await page.locator('#memoryClose').click();await expect(page.locator('#memoryDetail')).toHaveCount(0);
});

test("web graph offers compact layouts and fits every record", async ({page,project}) => {
 await graphView(page,project);
 await expect(page.locator('#memoryLayout')).toHaveValue('network');
 for (const layout of ['network','radial','columns']) {
  await page.locator('#memoryLayout').selectOption(layout);
  await page.locator('#memoryFit').click();
  const canvas=await page.locator('.memoryCanvas svg').boundingBox();
  for (const card of await page.locator('[data-node] rect').all()) {
   const box=(await card.boundingBox())!;
   expect(box.x).toBeGreaterThanOrEqual(canvas!.x);
   expect(box.y).toBeGreaterThanOrEqual(canvas!.y);
   expect(box.x+box.width).toBeLessThanOrEqual(canvas!.x+canvas!.width);
   expect(box.y+box.height).toBeLessThanOrEqual(canvas!.y+canvas!.height);
  }
 }
});
test("network spreads records in both dimensions and fits selected neighbours", async ({page,project}) => {
 await graphView(page,project);
 const xs=await page.locator('[data-node]').evaluateAll(es=>es.map(e=>e.getAttribute('transform')!.split(',')[0]));
 expect(new Set(xs).size).toBeGreaterThan(2);
 await page.locator('#memoryRecord').selectOption('t-1');
 await page.locator('#memoryFitSelection').click();
 await expect(page.locator('#memoryDetail')).toContainText('Build the Memory browser');
 await page.locator('#memoryLayout').selectOption('radial');
 await expect(page.locator('[data-node="t-1"]')).toHaveAttribute('aria-pressed','true');
});

test("large network fits without overlapping record cards",async({page,project})=>{
 const nodes=Array.from({length:57},(_,i)=>({...graph.nodes[i%5],id:'record-'+i,title:'Record '+i}));
 const edges=Array.from({length:42},(_,i)=>({...graph.edges[i%4],id:'link-'+i,source:'record-'+i,target:'record-'+((i+7)%57)}));
 await page.route('**/api/memory-graph',r=>r.fulfill({json:{...graph,nodes,edges}}));
 await open(page,project);await page.locator('[data-view="memory"]').click();
 await expect(page.locator('[data-node]')).toHaveCount(57);
 const result=await page.locator('.memoryCanvas svg').evaluate(svg=>{
  const frame=svg.getBoundingClientRect(),boxes=[...svg.querySelectorAll('[data-node] rect')].map(e=>e.getBoundingClientRect());
  return {outside:boxes.filter(b=>b.left<frame.left||b.right>frame.right||b.top<frame.top||b.bottom>frame.bottom).length,
   overlaps:boxes.flatMap((a,i)=>boxes.slice(i+1).filter(b=>Math.min(a.right,b.right)>Math.max(a.left,b.left)&&Math.min(a.bottom,b.bottom)>Math.max(a.top,b.top))).length};
 });
 expect(result).toEqual({outside:0,overlaps:0});
});

test("Memories use rounded cards and plain circular bead markers",async({page,project})=>{
 await graphView(page,project);
 await expect(page.locator('[data-node="m-a"] .memoryBead')).toHaveCount(1);
 const circle=page.locator('[data-node="m-a"] .memoryBead');
 await expect(circle).toHaveAttribute('fill','currentColor');
 await expect(page.locator('[data-node="m-a"] .memoryCard')).toHaveAttribute('rx','13');
 await expect(page.locator('radialGradient')).toHaveCount(0);
 const geometry=await page.locator('[data-edge="f"] > path').evaluate(path=>{
  const edge=path as SVGPathElement;
  const p=edge.getPointAtLength(edge.getTotalLength()),node=document.querySelector('[data-node="m-a"]') as SVGGElement;
  const m=node.transform.baseVal.consolidate()!.matrix,c=node.querySelector('.memoryCard') as SVGRectElement;
  const x=p.x-m.e,y=p.y-m.f;
  return Math.min(Math.abs(x),Math.abs(y),Math.abs(x-c.width.baseVal.value),Math.abs(y-c.height.baseVal.value));
 });
 expect(geometry).toBeCloseTo(0,1);
});
test("Labels button and l hide and show the Link labels",async({page,project})=>{
 await graphView(page,project);
 const labels=page.locator('.memoryEdgeLabel'),btn=page.locator('#memoryLabels');
 await expect(btn).toHaveAttribute('aria-pressed','true');
 await expect(labels.first()).toBeVisible();
 await btn.click();
 await expect(btn).toHaveAttribute('aria-pressed','false');
 for(const l of await labels.all())await expect(l).toBeHidden();
 await page.locator('#memoryType').selectOption('cites');await expect(labels).toHaveCount(1);await expect(labels).toBeHidden();
 await page.locator('[data-view="memory"]').focus();await page.keyboard.press('l');
 await expect(page.locator('#memoryLabels')).toHaveAttribute('aria-pressed','true');await expect(labels).toBeVisible();
});
test("graph canvas fills the Memory view to its bottom",async({page,project})=>{
 await graphView(page,project);
 const view=await page.locator('#vMemory').boundingBox(),svg=await page.locator('.memoryCanvas svg').boundingBox();
 // No gap below the canvas; a phone whose toolbar wraps keeps a minimum canvas and scrolls the view instead.
 const gap=view!.y+view!.height-(svg!.y+svg!.height);
 expect(gap).toBeLessThan(2);
 if(gap<-2)expect(svg!.height).toBeLessThan(262);
 const vb=(await page.locator('.memoryCanvas svg').getAttribute('viewBox'))!.split(' ').map(Number);
 expect(Math.abs(vb[3]-svg!.height)).toBeLessThan(2);
});
test("dragging a record moves it and its Links follow",async({page,project})=>{
 await graphView(page,project);
 const node=page.locator('[data-node="m-a"]'),edge=page.locator('[data-edge="f"] > path');
 const before=await node.getAttribute('transform'),d=await edge.getAttribute('d');
 const box=(await node.locator('.memoryCard').boundingBox())!;
 await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();
 await page.mouse.move(box.x+box.width/2+40,box.y+box.height/2+30,{steps:5});await page.mouse.up();
 await expect(node).not.toHaveAttribute('transform',before!);
 await expect(edge).not.toHaveAttribute('d',d!);
 await expect(page.locator('#memoryDetail')).toHaveCount(0);
 const moved=await node.getAttribute('transform');
 await page.locator('#memoryType').selectOption('follows');
 await expect(node).toHaveAttribute('transform',moved!);
});
test("M opens Memory from the tree and the board, and again returns to the tree",async({page,project})=>{
 await open(page,project as Parameters<typeof open>[1]);
 await page.keyboard.press('M');
 await expect(page.locator('#vMemory')).toBeVisible();await expect(page.locator('#vMemory [data-node]')).toHaveCount(5);
 await expect(page.locator('[data-view="memory"]')).toHaveAttribute('aria-current','page');
 await page.keyboard.press('M');await expect(page.locator('#vTree')).toBeVisible();
 await page.keyboard.press('b');await page.keyboard.press('M');await expect(page.locator('#vMemory')).toBeVisible();
 await page.keyboard.press('?');await expect(page.locator('#sheetHost .msheet kbd',{hasText:/^M$/})).toBeVisible();
});
test("d hides the Memory detail and shows the previous selection again",async({page,project})=>{
 await graphView(page,project);
 await page.locator('[data-view="memory"]').focus();await page.keyboard.press('d');
 await expect(page.locator('#memoryDetail')).toHaveCount(0);
 await page.locator('[data-node="m-a"] .memoryBead').click();await expect(page.locator('#memoryDetail')).toHaveAttribute('data-id','m-a');
 await page.keyboard.press('d');await expect(page.locator('#memoryDetail')).toHaveCount(0);
 await expect(page.locator('[data-node="m-a"]')).toHaveAttribute('aria-pressed','false');
 await page.keyboard.press('d');await expect(page.locator('#memoryDetail')).toHaveAttribute('data-id','m-a');
 await expect(page.locator('[data-node="m-a"]')).toHaveAttribute('aria-pressed','true');
 await page.keyboard.press('d');await page.locator('[data-edge="c"] .memoryEdgeLabel').click();await page.keyboard.press('Escape');
 await page.keyboard.press('d');await expect(page.locator('#memoryDetail')).toHaveAttribute('data-id','link:c');
});

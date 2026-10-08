const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {JSDOM}=require('jsdom');
const root=__dirname+'/';
function page(file){const dom=new JSDOM(fs.readFileSync(root+file,'utf8'),{runScripts:'outside-only',url:'https://example.invalid/'+file});Object.defineProperty(dom.window.SVGElement.prototype,'viewBox',{get(){const [x,y,width,height]=this.getAttribute('viewBox').split(' ').map(Number);return {baseVal:{x,y,width,height}};}});dom.window.eval(fs.readFileSync(root+'mockups.js','utf8')+'\nwindow.fixture=JSON.stringify({links,records});');return dom;}
test('fixture matches all installed Link Types and dependency contract',{timeout:5000},()=>{
 const dom=page('01-focus.html');const data=JSON.parse(dom.window.fixture);
 assert.deepEqual([...new Set(data.links.map(l=>l.type.split('/').at(-1)))].sort(),['example-cites','example-follows','preview-blocks-v1','preview-related-v2']);
 for(const l of data.links.filter(l=>l.type.endsWith('/preview-blocks-v1'))){assert.deepEqual(l.properties,{});const source=data.records.find(r=>r.id===l.source),target=data.records.find(r=>r.id===l.target);assert.ok(source.type.endsWith('/preview-issue-v2')&&target.type.endsWith('/preview-issue-v2'));assert.ok(source.owned.some(o=>o.id===l.id));}
 dom.window.close();
});
test('TUI wires show all four relations and permit Issue and Memory focus',{timeout:5000},()=>{
 const dom=page('01-focus.html'),d=dom.window.document;
 assert.ok(d.body.classList.contains('tui'));
 for(const name of ['follows','cites','related','depends on'])assert.ok(d.querySelector('.wire-tree').textContent.includes(name),name);
 assert.equal(d.querySelectorAll('.relationship-key [data-relation]').length,5);
 d.querySelector('[data-relation="preview-blocks-v1"]').click();assert.equal(d.querySelectorAll('.wire-branch').length,1);assert.match(d.querySelector('.wire-tree').textContent,/Build the Memory browser/);
 d.querySelector('[data-relation="all"]').click();d.querySelector('.wire-branch [data-link]').click();assert.match(d.querySelector('.inspector').textContent,/Selected Link/);
 d.querySelector('.record-rail [data-select$="embedded-read"]').click();assert.match(d.querySelector('.wire-root').textContent,/Read embedded issues/);assert.equal(d.querySelectorAll('.wire-branch[data-kind="preview-blocks-v1"]').length,0);
 d.querySelector('#canvas').dispatchEvent(new dom.window.KeyboardEvent('keydown',{key:'4',bubbles:true}));assert.equal(d.querySelectorAll('.wire-branch').length,0);
 d.querySelector('#canvas').dispatchEvent(new dom.window.KeyboardEvent('keydown',{key:'0',bubbles:true}));assert.ok(d.querySelectorAll('.wire-branch').length>0);
 d.querySelector('#canvas').dispatchEvent(new dom.window.KeyboardEvent('keydown',{key:'k',bubbles:true}));assert.match(d.querySelector('.wire-root').textContent,/Group work by time horizon/);
 dom.window.close();
});
test('web graph defaults to web and keeps stable node coordinates on selection',{timeout:5000},()=>{
 const dom=page('04-graph.html'),d=dom.window.document;
 assert.equal(d.body.classList.contains('tui'),false);assert.equal(d.querySelectorAll('.graph-node').length,12);
 const coords=[...d.querySelectorAll('.graph-node')].map(n=>n.getAttribute('transform'));
 const first=d.querySelector('.graph-node');first.dispatchEvent(new dom.window.KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
 assert.deepEqual([...d.querySelectorAll('.graph-node')].map(n=>n.getAttribute('transform')),coords);
 for(const type of ['example-cites','example-follows','preview-related-v2','preview-blocks-v1'])assert.ok(d.querySelector(`.graph-edge[data-kind="${type}"]`));
 d.querySelector('[data-relation="preview-blocks-v1"]').click();assert.equal(d.querySelectorAll('.graph-edge').length,1);assert.match(d.querySelector('.graph-edge').textContent,/depends on/);
 d.querySelector('[data-relation="all"]').click();d.querySelector('.edge-label').dispatchEvent(new dom.window.MouseEvent('click',{bubbles:true}));assert.match(d.querySelector('.inspector').textContent,/Selected Link/);
 d.querySelector('#zoomIn').click();const transform=d.querySelector('.topology').getAttribute('transform');
 d.querySelector('[data-tab="Body"]').click();d.querySelector('#follow').click();d.querySelector('#demoChange').click();assert.equal(d.querySelector('.topology').getAttribute('transform'),transform);
 d.querySelector('[data-tab="Versions"]').click();d.querySelector('[data-version="retained"]').click();const retained=d.querySelector('.inspector-body').textContent;
 d.querySelector('#demoChange').click();assert.equal(d.querySelector('.inspector-body').textContent,retained);
 d.querySelector('#overview').click();assert.equal(d.querySelectorAll('.graph-edge.dim').length,0);
 dom.window.close();
});

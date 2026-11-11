import datetime
import hashlib
import json
from pathlib import Path
import re
import sys
import xml.etree.ElementTree as ET
import yaml

root = Path('/home/omnigent/workspace/b9s')
path = root / 'docs/adr/0066-open-specification-references-with-explicit-snapshot-import.md'
text = path.read_text()
checks = []
def check(name, condition, detail):
    checks.append({'name': name, 'passed': bool(condition), 'detail': detail})

parts = text.split('---\n', 2)
fm = yaml.safe_load(parts[1])
check('frontmatter', fm == {'type':'ADR','id':'0066','title':'Open specification references with explicit snapshot import','status':'proposed','date':datetime.date(2026,11,11),'superseded_by':None}, 'YAML parsed with required ADR fields')
other_ids = [int(p.name[:4]) for p in path.parent.glob('[0-9][0-9][0-9][0-9]-*.md') if p != path]
check('number', int(fm['id']) > max(other_ids), f'new=66 previous maximum={max(other_ids)}')
headers = re.findall(r'^#{1,4} (.+)$', text, re.M)
check('sections', {'Context','Decision','Options considered','Consequences','Follow-up tasks','Advice'}.issubset(headers), headers)
check('toc', len(headers)<=10 or 'Contents' in headers, f'{len(headers)} headers, TOC required only above 10')
links = re.findall(r'\[[^\]]+\]\(([^)]+)\)', text)
local = [link for link in links if not re.match(r'https?://',link)]
check('local_links', all((path.parent / link.split('#')[0]).is_file() for link in local), local)
i = json.load(open('/tmp/bd-teri.13-evidence/issue-18-rest.json'))
comments = json.load(open('/tmp/bd-teri.13-evidence/issue-18-comments.json'))
check('issue_content', i['number']==18 and i['comments']==len(comments)==0 and not re.search(r'!\[|<img|user-attachments|\.png|\.jpe?g',i['body'],re.I), 'REST issue and complete comments inspected, 0 comments, no image references')
followups = json.load(open('/tmp/bd-teri.13-evidence/revision-2-follow-ups.json'))
by_id = {i['id']: i for i in followups}
expected = {'bd-cbs3','bd-psgc','bd-0alf','bd-6oqw'}
valid=True
for issue_id in expected:
    issue=by_id[issue_id]
    deps=issue.get('dependencies',[])
    valid = valid and issue['status']=='deferred' and any(dep.get('id',dep.get('depends_on_id'))=='bd-teri.13' and dep.get('dependency_type',dep.get('type'))=='discovered-from' for dep in deps)
check('followups',valid,'Four existing deferred tickets, each linked discovered-from:bd-teri.13')
check('text_rules', '\u2014' not in text and text.endswith('\n') and all(line == line.rstrip() for line in text.splitlines()), 'No em dashes or trailing whitespace, final newline present')
diagrams = re.findall(r'```mermaid\n(.*?)\n```', text, re.S)
diagram = diagrams[0] if len(diagrams) == 1 else ''
participants = re.findall(r'^\s*(?:actor|participant)\s+(\w+)', diagram, re.M)
open_branch = diagram.split('else Import snapshot', 1)[0]
required = ['alt Open reference', 'else Import snapshot', 'Guarded source access', 'Check project, root containment or HTTPS scheme', 'approved HTTPS origin under limits', 'alt Upload fails', 'else Comment fails', 'orphan', 'GC grace']
upload = diagram.find('App->>Blob:')
comment = diagram.find('App->>Bd:')
check('interaction_diagram', diagram.startswith('sequenceDiagram\n') and 3 <= len(participants) <= 6 and all(term in diagram for term in required) and 0 <= upload < comment and 'App->>Blob:' not in open_branch and 'App->>Bd:' not in open_branch, {'participants': participants, 'contracts': 'Explicit open versus import, guarded access, upload before comment, visible failures and orphan GC'})
check('diagram_label_rules', bool(diagram) and ';' not in diagram and re.search(r'&(?:#\w+|\w+);', diagram) is None and 'style ' not in diagram and 'classDef ' not in diagram, 'No semicolons, HTML entities or hardcoded fills in diagram')
manifest_path = Path('/tmp/bd-teri.13-evidence/revision-2-mermaid-render-final.json')
render_ok = False
render_detail = 'Render manifest missing'
if manifest_path.is_file():
    render = json.loads(manifest_path.read_text())
    svg_path = Path(render['output'])
    svg = svg_path.read_bytes()
    render_ok = render['exit_code'] == 0 and render['input_sha256'] == hashlib.sha256((diagram + '\n').encode()).hexdigest() and render['output_sha256'] == hashlib.sha256(svg).hexdigest() and ET.fromstring(svg).tag == '{http://www.w3.org/2000/svg}svg' and len(svg) > 1000
    render_detail = {'command': render['command'], 'input_sha256': render['input_sha256'], 'output_sha256': render['output_sha256'], 'bytes': len(svg)}
check('mermaid_render_evidence', render_ok, render_detail)
counts={'passed':sum(c['passed'] for c in checks),'failed':sum(not c['passed'] for c in checks),'skipped':0,'interrupted':0,'unrun':0}
report={'command':['python3',sys.argv[0]],'checks':checks,'counts':counts}
print(json.dumps(report,indent=2))
raise SystemExit(bool(counts['failed']))

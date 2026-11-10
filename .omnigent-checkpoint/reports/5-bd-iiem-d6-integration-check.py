import collections,difflib,json,pathlib,re,subprocess,sys,tempfile
repo=pathlib.Path('/home/omnigent/workspace/b9s')
phase=sys.argv[1]
assert phase in ['before','after']
def git(*args): return subprocess.check_output(['git','-C',str(repo),*args]).decode()
base=git('merge-base','origin/main','HEAD').strip()
paths=git('diff','--name-only',base).splitlines()
expected_paths=['Makefile','docs/USER_GUIDE.md','docs/testing.md','pkg/ui/issue_detail_test.go','tests/e2e/detail_footer_keys_e2e_test.go']
assert sorted(paths)==sorted(expected_paths)
root=pathlib.Path(tempfile.mkdtemp(prefix=f'bd-iiem-d6-{phase}-'))
report={'phase':phase,'head':git('rev-parse','HEAD').strip(),'assigned_main':git('rev-parse','origin/main').strip(),'merge_base':base,'snapshot_directory':str(root),'files':[]}
main={}; delivery={}; merged={}; merged_paths={}; checks={}; diffs=[]
for n,path in enumerate(paths):
 main[path]=git('show',f'origin/main:{path}'); delivery[path]=(repo/path).read_text()
 anc=root/f'{n}-base'; mf=root/f'{n}-main'; df=root/f'{n}-delivery'; out=root/f'{n}-integrated'
 anc.write_text(git('show',f'{base}:{path}')); mf.write_text(main[path]); df.write_text(delivery[path])
 argv=['git','merge-file','-p','-L','assigned-main','-L','merge-base','-L','delivery-worktree',str(mf),str(anc),str(df)]
 p=subprocess.run(argv,capture_output=True,text=True,timeout=30)
 out.write_text(p.stdout); merged[path]=p.stdout; merged_paths[path]=out
 markers=sum(l.startswith('<<<<<<< ') for l in p.stdout.splitlines())
 report['files'].append({'path':path,'command':argv,'exit_code':p.returncode,'conflict_markers':markers,'integrated_output':str(out),'stderr':p.stderr})
 checks[f'{path}:textual_merge']=p.returncode==0 and markers==0
 diffs.extend(difflib.unified_diff(main[path].splitlines(True),p.stdout.splitlines(True),fromfile='assigned-main/'+path,tofile='integrated/'+path))
make_reports=[]
for label,path,text in [('delivery',repo/'Makefile',delivery['Makefile']),('integrated',merged_paths['Makefile'],merged['Makefile'])]:
 counts={'stable_targets':len(re.findall(r'^test-stable\s*:',text,re.M)),'skip_definitions':len(re.findall(r'^STABLE_SKIP\s*:?=',text,re.M)),'stable_recipes':text.count("\tgo test -timeout 10m -skip '$(STABLE_SKIP)' $$(go list ./... | grep -v /tests/e2e)")}
 argv=['timeout','--kill-after=5s','30s','make','--no-print-directory','-C',str(repo),'-f',str(path),'-n','test-stable']
 p=subprocess.run(argv,capture_output=True,text=True,timeout=40)
 warning=bool(re.search(r'overriding.*recipe|ignoring.*recipe',p.stderr,re.I))
 record={'label':label,'counts':counts,'command':argv,'exit_code':p.returncode,'warnings':warning,'stdout':p.stdout,'stderr':p.stderr}
 make_reports.append(record)
 checks[label+':stable_uniqueness']=all(v==1 for v in counts.values())
 checks[label+':make_parse_warning_free']=p.returncode==0 and not warning and not p.stderr
checks['integrated_makefile_equals_main_verbatim']=merged['Makefile']==main['Makefile']
start='# The suite automated agents run.'; end='# The web UI'
checks['delivery_stable_semantics_match_main']=delivery['Makefile'][delivery['Makefile'].index(start):delivery['Makefile'].index(end)]==main['Makefile'][main['Makefile'].index(start):main['Makefile'].index(end)]
intro='`Enter` moves focus to the detail pane, and `Enter` or `Esc` moves it back. The focused pane has the bright border, and the footer shows its keys.'
full=' The header shows the full issue ID, including the project prefix.'
checks['guide_is_main_plus_only_full_id_sentence']=merged['docs/USER_GUIDE.md']==main['docs/USER_GUIDE.md'].replace(intro,intro+full,1)
schedule=next(p for p in main['docs/USER_GUIDE.md'].split('\n\n') if p.startswith('The pane opens with a card'))
checks['scheduling_paragraph_verbatim']=schedule in merged['docs/USER_GUIDE.md']
installer=next(p for p in main['docs/testing.md'].split('\n\n') if p.startswith("The source installer's Go requirement"))
checks['installer_paragraph_verbatim']=installer in merged['docs/testing.md']
stable=next(p for p in delivery['docs/testing.md'].split('\n\n') if p.startswith('Automated agents run `make test-stable` alone'))
e2e=next(p for p in main['docs/testing.md'].split('\n\n') if p.startswith('The E2E suite runs its PTY tests'))
testing_expected=main['docs/testing.md'].replace('go build ./...\n','go build ./...\nmake test-stable                           # automated agents, bounded packages and host-sensitive exclusions\n',1).replace(e2e,e2e+'\n\n'+stable,1)
checks['testing_is_main_plus_only_stable_instructions']=merged['docs/testing.md']==testing_expected

def function_block(text,name):
 a=text.index('func '+name+'('); b=text.find('\nfunc ',a+1)
 return text[a:] if b<0 else text[a:b+1]
ui='pkg/ui/issue_detail_test.go'; pty='tests/e2e/detail_footer_keys_e2e_test.go'
viewport=function_block(delivery[ui],'TestIssueDetailViewportShowsFullID')
ui_expected=main[ui].replace('[]string{"e5u3.9",','[]string{"bd-e5u3.9",').replace('func TestRenderIssueDetail_ListsRelationsBothWays(',viewport+'func TestRenderIssueDetail_ListsRelationsBothWays(',1)
checks['ui_is_main_plus_only_full_id_assertions_and_viewport_test']=merged[ui]==ui_expected
pty_block=function_block(delivery[pty],'TestDetailHeaderShowsFullIDE2E')
pty_expected=main[pty].replace('\t"time"\n)','\t"time"\n\n\t"github.com/charmbracelet/x/ansi"\n)',1).replace('func TestDetailEKeyOpensEditModalE2E(',pty_block+'func TestDetailEKeyOpensEditModalE2E(',1)
checks['pty_is_main_plus_only_full_id_regression_and_import']=merged[pty]==pty_expected
checks['long_id_case_retained']='"long-project-name-iiem.2.3"' in merged[ui]
checks['visible_header_assertion_retained']='header := out[top:bottom]' in merged[ui] and 'strings.Contains(header, want)' in merged[ui]
checks['pty_full_id_assertion_retained']='[]string{"detail-1", "Detail issue", "Some description."}' in merged[pty]
for path in [ui,pty]:
 funcs=re.findall(r'^func ([A-Za-z0-9_]+)\(',merged[path],re.M)
 checks[path+':unique_functions']=all(v==1 for v in collections.Counter(funcs).values())
 checks[path+':main_functions_retained']=set(re.findall(r'^func ([A-Za-z0-9_]+)\(',main[path],re.M))<=set(funcs)
 p=subprocess.run(['gofmt','-d',str(merged_paths[path])],capture_output=True,text=True,timeout=30)
 checks[path+':integrated_gofmt_clean']=p.returncode==0 and not p.stdout and not p.stderr
report['make_checks']=make_reports; report['assertions']=checks; report['passed']=sum(checks.values()); report['failed']=sum(not x for x in checks.values()); report['clean']=all(checks.values())
report_path=pathlib.Path(f'/tmp/bd-iiem-d6-{phase}-integration.json')
report_path.write_text(json.dumps(report,indent=2)+'\n')
pathlib.Path(f'/tmp/bd-iiem-d6-{phase}-integration.diff').write_text(''.join(diffs))
for f in report['files']: print(f"{f['path']}: merge exit={f['exit_code']}, conflict markers={f['conflict_markers']}")
for m in make_reports:
 print(f"{m['label']} Makefile: {m['counts']}, make exit={m['exit_code']}, warnings={m['warnings']}")
 if m['stderr']: print(m['stderr'].strip())
for k,v in checks.items():
 if not v: print('FAILED ASSERTION: '+k)
print(f"Content/integration assertions: passed={report['passed']} failed={report['failed']}")
raise SystemExit(0 if report['clean'] else 1)

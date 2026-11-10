import hashlib,json,os,pathlib,subprocess,tempfile
repo=pathlib.Path('/home/omnigent/workspace/b9s')
root=pathlib.Path(tempfile.mkdtemp(prefix='bd-iiem-d6-tree-'))
objects=root/'objects'; objects.mkdir()
commands=[]
def run(*args,env=None,data=None):
 argv=['git','-C',str(repo),*args]
 p=subprocess.run(argv,env=env,input=data,capture_output=True,timeout=60)
 commands.append({'argv':argv,'exit_code':p.returncode,'stderr':p.stderr.decode()})
 if p.returncode: raise RuntimeError((argv,p.returncode,p.stdout.decode(),p.stderr.decode()))
 return p.stdout
head=run('rev-parse','HEAD').decode().strip()
refs=run('for-each-ref','--format=%(refname) %(objectname)')
index=pathlib.Path(run('rev-parse','--path-format=absolute','--git-path','index').decode().strip())
index_before=hashlib.sha256(index.read_bytes()).hexdigest()
semantic=json.loads(pathlib.Path('/tmp/bd-iiem-d6-after-integration.json').read_text())
paths=[f['path'] for f in semantic['files']]
files_before={p:hashlib.sha256((repo/p).read_bytes()).hexdigest() for p in paths}
env=os.environ.copy()
env.update({'GIT_INDEX_FILE':str(root/'index'),'GIT_OBJECT_DIRECTORY':str(objects),'GIT_ALTERNATE_OBJECT_DIRECTORIES':run('rev-parse','--path-format=absolute','--git-path','objects').decode().strip(),'GIT_AUTHOR_NAME':'Integration check','GIT_AUTHOR_EMAIL':'integration@example.invalid','GIT_COMMITTER_NAME':'Integration check','GIT_COMMITTER_EMAIL':'integration@example.invalid'})
run('read-tree','HEAD',env=env)
run('add','--',*paths,env=env)
tree=run('write-tree',env=env).decode().strip()
synthetic=run('commit-tree',tree,'-p',head,env=env,data=b'Temporary integration check\n').decode().strip()
merged_output=run('merge-tree','--write-tree','origin/main',synthetic,env=env).decode()
integrated=merged_output.splitlines()[0]
assert len(merged_output.splitlines())==1,merged_output
checks={}
for f in semantic['files']:
 content=run('show',f'{integrated}:{f["path"]}',env=env)
 checks[f['path']+':matches_reviewed_file_integration']=content==pathlib.Path(f['integrated_output']).read_bytes()
main_make=run('show','origin/main:Makefile')
merged_make=run('show',f'{integrated}:Makefile',env=env)
make_path=root/'integrated.Makefile'; make_path.write_bytes(merged_make)
checks['actual_integrated_makefile_equals_main']=merged_make==main_make
argv=['timeout','--kill-after=5s','30s','make','--no-print-directory','-C',str(repo),'-f',str(make_path),'-n','test-stable']
p=subprocess.run(argv,capture_output=True,text=True,timeout=40)
checks['actual_integrated_make_parse_warning_free']=p.returncode==0 and not p.stderr
changed=run('diff','--name-only','origin/main',integrated,env=env).decode().splitlines()
checks['actual_main_diff_only_expected_four_task_files']=sorted(changed)==sorted([p for p in paths if p!='Makefile'])
checks['real_head_unchanged']=run('rev-parse','HEAD').decode().strip()==head
checks['real_refs_unchanged']=run('for-each-ref','--format=%(refname) %(objectname)')==refs
checks['real_index_unchanged']=hashlib.sha256(index.read_bytes()).hexdigest()==index_before
checks['real_task_files_unchanged']=all(hashlib.sha256((repo/p).read_bytes()).hexdigest()==v for p,v in files_before.items())
report={'root':str(root),'temporary_git_environment':{k:env[k] for k in ['GIT_INDEX_FILE','GIT_OBJECT_DIRECTORY','GIT_ALTERNATE_OBJECT_DIRECTORIES']},'source_head':head,'synthetic_commit':synthetic,'integrated_tree':integrated,'merge_tree_output':merged_output,'commands':commands,'make_parse':{'argv':argv,'exit_code':p.returncode,'stdout':p.stdout,'stderr':p.stderr},'diff_paths_against_main':changed,'assertions':checks,'clean':all(checks.values())}
pathlib.Path('/tmp/bd-iiem-d6-tree-integration.json').write_text(json.dumps(report,indent=2)+'\n')
for k,v in checks.items(): print(f'{k}={v}')
print('actual_merge_tree_clean='+str(report['clean']))
raise SystemExit(0 if report['clean'] else 1)

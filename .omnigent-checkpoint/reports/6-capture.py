import codecs, fcntl, json, os, pty, select, signal, struct, subprocess, sys, termios, time
from pathlib import Path
import pyte
binary, label = sys.argv[1:]
out = Path('/tmp/bd-rz2i')
env = {'PATH': os.environ['PATH'], 'HOME': '/tmp/bd-rz2i', 'TERM': 'xterm-256color', 'COLORTERM':'truecolor', 'XDG_CONFIG_HOME': os.environ.get('B9S_CAPTURE_CONFIG','/tmp/bd-rz2i/config'), 'B9S_TEST_MODE':'1', 'B9S_NO_BROWSER':'1', 'B9S_TUI_AUTOCLOSE_MS':'3500', 'BEADS_DOLT_AUTO_START':'0'}
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH',40,160,0,0))
p = subprocess.Popen([binary], stdin=slave, stdout=slave, stderr=slave, cwd=out/'fixture', env=env, start_new_session=True)
os.close(slave)
screen = pyte.Screen(160,40)
stream = pyte.Stream(screen)
decoder = codecs.getincrementaldecoder('utf-8')('replace')
raw = bytearray()
passed=[]
long=os.environ.get('B9S_CAPTURE_LONG')=='1'
def pump(seconds):
    end = time.monotonic()+seconds
    while time.monotonic()<end:
        if select.select([master],[],[],0.05)[0]:
            try: data=os.read(master,65536)
            except OSError: return
            raw.extend(data)
            stream.feed(decoder.decode(data))
def wait_for(text):
    end=time.monotonic()+8
    while time.monotonic()<end:
        if text in '\n'.join(screen.display): return
        if p.poll() is not None: raise RuntimeError(f'exit {p.returncode}')
        pump(.1)
    raise RuntimeError(f'missing {text}:\n'+'\n'.join(screen.display))
def capture(name):
    pump(.3)
    lines=screen.display
    (out/f'{label}-{name}.txt').write_text('\n'.join(lines)+'\n')
    from PIL import Image, ImageDraw, ImageFont
    image=Image.new('RGB',(1600,800),'#151515')
    draw=ImageDraw.Draw(image)
    font=ImageFont.truetype('/usr/share/fonts/truetype/freefont/FreeMono.ttf',16)
    for y in range(40):
        for x in range(160):
            cell=screen.buffer[y][x]
            fg='#'+cell.fg if len(cell.fg)==6 else '#d4d4d4'
            if cell.data: draw.text((x*10,y*20),cell.data,font=font,fill=fg)
    image.save(out/f'{label}-{name}.png')
    if name=='top':
        right=lines[0].rfind('┐')
        assert right>150, 'outer border missing'
        for y,line in enumerate(lines[1:38],1):
            assert screen.buffer[y][right].data=='│', f'outer border shifted at row {y}: {line}'
            assert not ''.join(screen.buffer[y][x].data for x in range(right+1,160)).strip(), f'stray marks at row {y}: {line}'
        assert 'Ctrl-T    Open epics' in '\n'.join(lines)
    return lines
try:
    wait_for('Root')
    os.write(master,b'?')
    wait_for('Keyboard Shortcuts')
    capture('top')
    passed.append('top borders and original hotkey')
    os.write(master,b'G')
    pump(.4)
    bottom=capture('bottom')
    assert 'Wide columns' in '\n'.join(bottom) and 'Detail View' in '\n'.join(bottom), 'bottom content clipped'
    passed.append('scroll to bottom')
    os.write(master,b'\x13ctrl-t\r')
    wait_for('Open epics')
    lines=capture('hotkey')
    if long:
        row=next(y for y in range(40) if 'Ctrl-T' in lines[y])
        key_x=next(x for x in range(160) if ''.join(screen.buffer[row][c].data for c in range(x,min(x+6,160)))=='Ctrl-T')
        assert ''.join(screen.buffer[row+1][x].data for x in range(key_x,key_x+10)).strip()=='', 'continuation entered key column'
        continuation=''.join(screen.buffer[row+1][x].data for x in range(key_x+10,160))
        assert continuation.startswith('(:epic status:open)'), 'command continuation clipped'
    else:
        assert any('Ctrl-T    Open epics (:epic status:open)' in line for line in lines), 'hotkey clipped'
    passed.append('filtered hotkey containment and alignment')
    deadline=time.monotonic()+5
    while p.poll() is None and time.monotonic()<deadline: pump(.1)
    assert p.poll() is not None, 'TUI did not auto-close within exit deadline'
    assert p.returncode==0, f'TUI exit {p.returncode}'
    passed.append('bounded normal exit')
    print(json.dumps({'counts':{'passed':len(passed),'failed':0,'skipped':0,'interrupted':0,'unrun':0},'checks':passed,'binary':binary,'size':[160,40],'exit_code':p.returncode,'captures':['top','bottom','hotkey']}))
finally:
    (out/f'{label}.terminal').write_bytes(raw)
    if p.poll() is None: os.killpg(p.pid,signal.SIGTERM); p.wait(timeout=3)
    os.close(master)

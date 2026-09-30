import re, pathlib, collections
root=pathlib.Path('.')
files=[p for p in root.rglob('*.go') if '.kilo' not in str(p)]
text={}
for p in files:
    try: text[p]=p.read_text(encoding='utf-8-sig')
    except Exception: pass
alltext='\n'.join(text.values())
types=set(re.findall(r'type (Update\w+Request|Update\w+) struct', alltext))
for t in sorted(types):
    n=len(re.findall(r'\b'+t+r'\b', alltext))
    print(f'{t:38s} refs={n}')

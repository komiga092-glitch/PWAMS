import re, pathlib
root=pathlib.Path('.')
files=[p for p in root.rglob('*.go') if '.kilo' not in str(p)]
alltext='\n'.join(p.read_text(encoding='utf-8-sig') for p in files)
for name in ['ErrLoanCannotBeEdited','UpdateLoanRequest','ErrAidRequestCannotBeEdited','ErrCareProvidedCannotBeEdited']:
    print(name, len(re.findall(r'\b'+name+r'\b', alltext)))

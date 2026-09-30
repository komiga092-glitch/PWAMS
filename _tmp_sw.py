import json
s=json.load(open('docs/swagger.json',encoding='utf-8-sig'))
for p in sorted(s['paths']):
    if 'loan' in p:
        print(p, list(s['paths'][p].keys()))

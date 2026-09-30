import json
rows=json.load(open('qa_results.json',encoding='utf-8'))
print(type(rows), len(rows))
if isinstance(rows,dict):
    print(list(rows.keys())[:20])
else:
    print(json.dumps(rows[0],indent=1)[:800])

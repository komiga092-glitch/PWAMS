import json
rows=json.load(open('qa_results.json',encoding='utf-8'))
fails=[(k,v) for k,v in rows.items() if v.get('status')!='Pass']
print('total',len(rows),'fails',len(fails))
for k,v in fails:
    print('=====',k,v.get('status'))
    print(v.get('detail') or v.get('message') or '')

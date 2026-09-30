import json
cases=json.load(open('qa_testcases.json',encoding='utf-8'))
print(type(cases))
if isinstance(cases,dict):
    ks=list(cases.keys()); print(ks[:10])
    for k in ks[:3]:
        print(json.dumps(cases[k],indent=1)[:600])
else:
    print(len(cases)); print(json.dumps(cases[0],indent=1)[:600])

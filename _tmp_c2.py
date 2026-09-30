import json
cases=json.load(open('qa_testcases.json',encoding='utf-8'))
want={'AUTH-011','USR-003','DASH-004','PER-008','PER-010','AID-008','AID-010','LON-003','LON-009','LON-010','LON-011','CARE-003','RPT-003','SYNC-002'}
found={}
for sheet,rows in cases.items():
    for r in rows:
        if r[0] in want:
            found[r[0]]=(sheet,r)
for k in sorted(want):
    if k in found:
        sheet,r=found[k]
        print('=====',k,'['+sheet+']')
        print(' title   :',r[1])
        print(' pre     :',r[2])
        print(' steps   :',r[3].replace('\\n','; '))
        print(' input   :',r[4])
        print(' expected:',r[5])
    else:
        print('=====',k,'NOT IN WORKBOOK')

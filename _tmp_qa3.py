import json
rows=json.load(open('qa_results.json',encoding='utf-8'))
for k in ['AID-008','AID-010','CARE-003','DASH-005','LON-003','PER-008','PER-010','DASH-004','RPT-003','SYNC-002','USR-003','AUTH-011','LON-009','LON-010','LON-011']:
    v=rows.get(k)
    print('=====',k, json.dumps(v,indent=1)[:1400] if v else 'MISSING')

import json
d=json.load(open('internal/i18n/locales/en.json',encoding='utf-8'))
print(len(d))
for pre in ['common.','loans.','aid_requests.','donations.','donors.','persons.','students.','nav.']:
    ks=[k for k in d if k.startswith(pre)]
    print('---',pre,len(ks))
    print(', '.join(ks))

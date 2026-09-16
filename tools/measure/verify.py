import os, collections
from pypdf import PdfReader
rows=[l.split() for l in open('/work/declared.txt') if l.strip()]
c=collections.Counter(); bad=[]
for kind, docid, declared, blanks in rows:
    declared=int(declared)
    p=f'/work/pdfs/{docid}.pdf'
    if not os.path.exists(p) or os.path.getsize(p)==0:
        c[kind+':empty']+=1; bad.append((kind,docid,declared,0,'empty')); continue
    try:
        n=len(PdfReader(p).pages)
    except Exception as e:
        c[kind+':unreadable']+=1; bad.append((kind,docid,declared,-1,str(e)[:60])); continue
    if n==declared: c[kind+':exact']+=1
    else:
        c[kind+':mismatch']+=1; bad.append((kind,docid,declared,n,f'declared {declared} rendered {n}'))
print('documents checked:', len(rows))
for k,v in sorted(c.items()): print(f'  {k:22} {v}')
if bad:
    print('\nproblems:')
    for b in bad: print('  ', b[0], b[1], b[4])

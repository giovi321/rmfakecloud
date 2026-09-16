# Document identifiers in this file were redacted. Substitute a real
# rmfakecloud document id for the DOCUMENT-ID placeholders before running.
import os, collections, itertools
from pypdf import PdfReader
rows=[l.split() for l in open('/work/declared.txt') if l.strip()]
c=collections.Counter(); bad=[]
for kind, docid, declared in rows:
    declared=int(declared)
    p=f'/work/pdfs/{docid}.pdf'
    if not os.path.exists(p) or os.path.getsize(p)==0:
        c[kind+':empty']+=1; bad.append((kind,docid,'empty')); continue
    try:
        n=len(PdfReader(p).pages)
    except Exception as e:
        c[kind+':unreadable']+=1; bad.append((kind,docid,str(e)[:60])); continue
    if n==declared: c[kind+':exact']+=1
    else:
        c[kind+':mismatch']+=1; bad.append((kind,docid,f'declared {declared} rendered {n}'))
print('documents checked:', len(rows))
for k,v in sorted(c.items()): print(f'  {k:24} {v}')
if bad:
    print('\nproblems:')
    for b in bad: print('  ', b[0], b[1], b[2])

# the previously concatenated presentation: document kept, no appended ink pages
p='/work/pdfs/DOCUMENT-ID-1.pdf'
if os.path.exists(p):
    r=PdfReader(p)
    seq=[(round(float(x.mediabox.width)), round(float(x.mediabox.height))) for x in r.pages]
    runs=[(k,len(list(g))) for k,g in itertools.groupby(seq)]
    text=sum(1 for x in r.pages if (x.extract_text() or '').strip())
    print('\nDOCUMENT-ID-1 (was 51 document pages + 19 ink pages):')
    print('   pages', len(seq), 'size runs', runs, 'pages with text', text)

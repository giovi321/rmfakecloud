from pypdf import PdfReader
import itertools
for n in ('lez-tablet','lez-rmfc'):
    r=PdfReader(f'/cmp/{n}.pdf')
    seq=[(round(float(p.mediabox.width),1), round(float(p.mediabox.height),1), int(p.get('/Rotate',0))) for p in r.pages]
    runs=[(k,len(list(g))) for k,g in itertools.groupby(seq)]
    print(f'{n:11} pages={len(seq)} runs={runs[:5]}')

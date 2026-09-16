from pypdf import PdfReader
import itertools
for n in ('tablet','rmfc','lezione'):
    r=PdfReader(f'/cmp/{n}.pdf')
    seq=[]
    for p in r.pages:
        mb=p.mediabox
        rot=p.get('/Rotate', 0)
        seq.append((round(float(mb.width)), round(float(mb.height)), int(rot)))
    runs=[(k,len(list(g))) for k,g in itertools.groupby(seq)]
    print(f'{n:9} pages={len(seq)}  (w,h,rotate) runs={runs[:6]}')

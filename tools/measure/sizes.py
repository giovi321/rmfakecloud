import glob, itertools
from pypdf import PdfReader
for p in sorted(glob.glob('/check/a-*.pdf')):
    r=PdfReader(p)
    seq=[(round(float(x.mediabox.width)), round(float(x.mediabox.height))) for x in r.pages]
    runs=[(k,len(list(g))) for k,g in itertools.groupby(seq)]
    print(p.split('/')[-1][:14], 'pages', len(seq), 'sizes', runs[:3])

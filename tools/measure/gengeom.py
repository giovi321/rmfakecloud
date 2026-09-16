from pypdf import PdfReader
import itertools
for n in ('gen-export','gen-payload'):
    r=PdfReader(f'/check/{n}.pdf')
    seq=[(round(float(p.mediabox.width),1), round(float(p.mediabox.height),1)) for p in r.pages]
    print(n, 'pages', len(seq), 'sizes', [(k,len(list(g))) for k,g in itertools.groupby(seq)])

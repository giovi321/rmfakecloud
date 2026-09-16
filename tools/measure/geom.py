# Document identifiers in this file were redacted. Substitute a real
# rmfakecloud document id for the DOCUMENT-ID placeholders before running.
from pypdf import PdfReader
import itertools
d='DOCUMENT-ID'
for label, path in (('payload', f'/check/payload-{d}.pdf'), ('annotations', f'/check/annot-{d}.pdf')):
    r = PdfReader(path)
    seq = [(round(float(p.mediabox.width),1), round(float(p.mediabox.height),1)) for p in r.pages]
    runs = [(k, len(list(g))) for k, g in itertools.groupby(seq)]
    print(f'{label:12} pages={len(seq)}  runs={runs}')
    if seq:
        w,h = seq[0]
        print(f'             first page aspect h/w = {h/w:.4f}')
print('device canvas 1404x1872 aspect =', 1872/1404)

# Document identifiers in this file were redacted. Substitute a real
# rmfakecloud document id for the DOCUMENT-ID placeholders before running.
from pypdf import PdfReader
import itertools
for name in ('DOCUMENT-ID-1','DOCUMENT-ID-2'):
    r = PdfReader(f'/check/{name}.pdf')
    seq = [(round(float(p.mediabox.width)), round(float(p.mediabox.height))) for p in r.pages]
    runs = [(k, len(list(g))) for k, g in itertools.groupby(seq)]
    print(f'{name[:8]} pages={len(seq)}')
    print(f'   size runs in page order: {runs}')

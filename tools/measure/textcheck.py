# Document identifiers in this file were redacted. Substitute a real
# rmfakecloud document id for the DOCUMENT-ID placeholders before running.
from pypdf import PdfReader
for name in ('DOCUMENT-ID-1','DOCUMENT-ID-2'):
    r = PdfReader(f'/check/{name}.pdf')
    pattern = ''.join('T' if (pg.extract_text() or '').strip() else '.' for pg in r.pages)
    sizes = [(round(float(p.mediabox.width)), round(float(p.mediabox.height))) for p in r.pages]
    uniq = sorted(set(sizes))
    print(f'{name[:8]}  pages={len(r.pages)}')
    print(f'   text per page (T=has text): {pattern}')
    print(f'   distinct page sizes: {uniq}')

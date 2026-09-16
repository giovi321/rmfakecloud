import glob, sys
from pypdf import PdfReader
for p in sorted(glob.glob('/check/*.pdf')):
    try:
        r = PdfReader(p)
        sizes = {(round(float(pg.mediabox.width),1), round(float(pg.mediabox.height),1)) for pg in r.pages}
        print(f"{p.split('/')[-1][:12]}  pages={len(r.pages)}  mediaboxes={sorted(sizes)}")
    except Exception as e:
        print(f"{p.split('/')[-1][:12]}  ERROR {e}")

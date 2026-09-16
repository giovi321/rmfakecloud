from pypdf import PdfReader
import glob
for p in sorted(glob.glob('/check/big-*.pdf')):
    try:
        r = PdfReader(p)
        print(f'{p.split("/")[-1][:20]} pages={len(r.pages)}')
        sizes = {(round(float(pg.mediabox.width)), round(float(pg.mediabox.height))) for pg in r.pages}
        print('   sizes:', sorted(sizes))
    except Exception as e:
        print(f'{p.split("/")[-1][:20]} ERROR: {e}')

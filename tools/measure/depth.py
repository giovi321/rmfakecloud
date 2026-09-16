from pypdf import PdfReader
for name in ('deep.pdf','deep-optimized.pdf'):
    try:
        r = PdfReader('/check/'+name)
        print(f'{name}: pages={len(r.pages)}')
    except Exception as e:
        print(f'{name}: ERROR {e}')

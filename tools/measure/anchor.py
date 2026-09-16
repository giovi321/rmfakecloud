import pypdfium2 as pdfium
from PIL import ImageOps
for n in ('tl-0','bl-0','c-0','bl-plus'):
    try:
        d=pdfium.PdfDocument(f'/check/anchor-{n}.pdf')
    except Exception as e:
        print(n,'ERROR',e); continue
    page=d[0]
    w,h=page.get_size()
    img=page.render(scale=1.0).to_pil().convert('L')
    inv=ImageOps.invert(img)
    box=inv.getbbox()
    if box is None:
        print(f'{n:9} page {w}x{h}  no ink')
        continue
    # convert from image coords (y down) to pdf points (y up)
    x0,y0,x1,y1 = box
    print(f'{n:9} page {w:.0f}x{h:.0f}  ink x {x0}..{x1}  y_from_top {y0}..{y1}  (pdf y {h-y1:.0f}..{h-y0:.0f})')

import pypdfium2 as pdfium
import numpy as np

S = 3.0
d = pdfium.PdfDocument('/cmp/ink4.pdf')[0]
W, H = d.get_size()
img = np.asarray(d.render(scale=S).to_pil().convert('L'), dtype=np.int16)
ink = img < 200
ys, xs = np.nonzero(ink)
x0, x1 = xs.min()/S, xs.max()/S
y0, y1 = ys.min()/S, ys.max()/S
print(f'rmc-go page: {W:.1f} x {H:.1f} pt')
print(f'  ink bbox in that page: x {x0:.2f}..{x1:.2f}  y {y0:.2f}..{y1:.2f}')
print(f'  ink size: {x1-x0:.2f} x {y1-y0:.2f} pt')

# tablet measurements, page coordinates with y from the top of the document page
tx0, tx1 = 77.7, 845.7
ty0, ty1 = -28.67, 455.3
print(f'tablet ink size: {tx1-tx0:.2f} x {ty1-ty0:.2f} pt')
sx = (tx1-tx0)/(x1-x0)
sy = (ty1-ty0)/(y1-y0)
print(f'  scale x {sx:.5f}   scale y {sy:.5f}   ratio {sx/sy:.5f}')
print(f'  if uniform, the rmc-go page maps to {W*sx:.1f} x {H*sy:.1f} pt')
# offset: where rmc-go page origin lands on the document page
print(f'  rmc-go origin lands at x {tx0 - x0*sx:.2f}, y {ty0 - y0*sy:.2f}')

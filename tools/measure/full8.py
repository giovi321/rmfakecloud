import pypdfium2 as pdfium
import numpy as np

S = 3.0
tab = pdfium.PdfDocument('/cmp/lez-tablet.pdf')[0]
pay = pdfium.PdfDocument('/cmp/lez-payload.pdf')[7]
a = np.asarray(tab.render(scale=S).to_pil().convert('L'), dtype=np.int16)
p = np.asarray(pay.render(scale=S).to_pil().convert('L'), dtype=np.int16)
extra = a.shape[0] - p.shape[0]

best=None
for dy in range(0, extra+1):
    w = a[dy:dy+p.shape[0], :p.shape[1]]
    r = int(np.abs(w-p).sum())
    if best is None or r < best[1]: best=(dy,r)
dy = best[0]
print(f'page top sits at tablet row {dy} ({dy/S:.2f}pt), extension {extra/S:.2f}pt')

mask = np.zeros(a.shape, dtype=bool)
# above the page: plain dark pixels, nothing printed there
mask[:dy, :] = a[:dy, :] < 200
# over the page: whatever differs from the original
w = a[dy:dy+p.shape[0], :p.shape[1]]
mask[dy:dy+p.shape[0], :p.shape[1]] = np.abs(w-p) > 60
# below the page, if any
if dy+p.shape[0] < a.shape[0]:
    mask[dy+p.shape[0]:, :] = a[dy+p.shape[0]:, :] < 200

ys, xs = np.nonzero(mask)
x0,x1 = xs.min()/S, xs.max()/S
y0,y1 = ys.min()/S - dy/S, ys.max()/S - dy/S
print(f'tablet ink, page coords: x {x0:.1f}..{x1:.1f}  y {y0:.1f}..{y1:.1f}   size {x1-x0:.1f} x {y1-y0:.1f}')

# true device extent from decoding the .rm
dx0,dx1, dy0,dy1 = 229.5, 1317.7, 127.6, 1281.2
sx = (x1-x0)/(dx1-dx0); sy = (y1-y0)/(dy1-dy0)
print(f'scale x {sx:.5f} pt/px   scale y {sy:.5f} pt/px   (72/226 = {72/226:.5f})')
print(f'offsets: Cx {x0-dx0*sx:.2f}  Cy {y0-dy0*sy:.2f}')
print(f'page half width 480; 702*sx = {702*sx:.2f}')

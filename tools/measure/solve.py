import pypdfium2 as pdfium
from PIL import Image, ImageChops
import numpy as np

S = 3.0  # render scale, px per point

tab = pdfium.PdfDocument('/cmp/tablet.pdf')[0].render(scale=S).to_pil().convert('L')
pay = pdfium.PdfDocument('/cmp/payload.pdf')[3].render(scale=S).to_pil().convert('L')
print('tablet px', tab.size, 'payload px', pay.size)

ta = np.asarray(tab, dtype=np.int16)
pa = np.asarray(pay, dtype=np.int16)

# the tablet page is taller; find the vertical shift that cancels the print
best=None
extra = ta.shape[0] - pa.shape[0]
for dy in range(0, extra+1):
    window = ta[dy:dy+pa.shape[0], :pa.shape[1]]
    residual = int(np.abs(window - pa).sum())
    if best is None or residual < best[1]:
        best = (dy, residual)
dy, residual = best
print(f'best vertical shift: tablet row {dy} lines up with payload row 0  (extra rows {extra})')

window = ta[dy:dy+pa.shape[0], :pa.shape[1]]
diff = np.abs(window - pa)
ink = diff > 60
ys, xs = np.nonzero(ink)
print('ink pixels:', len(xs))
x0,x1 = xs.min()/S, xs.max()/S
y0,y1 = ys.min()/S, ys.max()/S
print(f'ink bbox on the page, in points: x {x0:.1f}..{x1:.1f}  y_from_top {y0:.1f}..{y1:.1f}')
print(f'ink size: {x1-x0:.1f} x {y1-y0:.1f}')

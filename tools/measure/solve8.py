import pypdfium2 as pdfium
import numpy as np

S = 3.0
def ink_bbox(annotated_page, payload_page, label):
    a = np.asarray(annotated_page.render(scale=S).to_pil().convert('L'), dtype=np.int16)
    p = np.asarray(payload_page.render(scale=S).to_pil().convert('L'), dtype=np.int16)
    extra = a.shape[0] - p.shape[0]
    best = None
    for dy in range(0, max(extra, 0) + 1):
        w = a[dy:dy+p.shape[0], :p.shape[1]]
        r = int(np.abs(w - p).sum())
        if best is None or r < best[1]:
            best = (dy, r)
    dy = best[0]
    w = a[dy:dy+p.shape[0], :p.shape[1]]
    diff = np.abs(w - p) > 60
    ys, xs = np.nonzero(diff)
    if len(xs) == 0:
        print(label, 'no ink found'); return None
    x0, x1 = xs.min()/S, xs.max()/S
    y0, y1 = ys.min()/S, ys.max()/S
    print(f'{label}: page grew {extra/S:.2f}pt at the top, ink x {x0:.1f}..{x1:.1f}  y_from_page_top {y0-dy/S:.1f}..{y1-dy/S:.1f}')
    return (x0, x1, y0 - dy/S, y1 - dy/S)

pay = pdfium.PdfDocument('/cmp/lez-payload.pdf')[7]
tab = pdfium.PdfDocument('/cmp/lez-tablet.pdf')[0]
rmf = pdfium.PdfDocument('/cmp/lez-rmfc.pdf')[7]
print('payload page', pay.get_size(), 'tablet', tab.get_size(), 'rmfakecloud', rmf.get_size())

t = ink_bbox(tab, pay, 'tablet     ')
r = ink_bbox(rmf, pay, 'rmfakecloud')

if t and r:
    SCALE = 72/226
    OLD = 960/1404          # the scale the v5 path uses for this page
    # invert rmfakecloud's own transform to recover device pixels
    dx0, dx1 = r[0]/OLD, r[1]/OLD
    dy0, dy1 = r[2]/OLD, r[3]/OLD
    print(f'\nink in device pixels: x {dx0:.1f}..{dx1:.1f}  y {dy0:.1f}..{dy1:.1f}')
    print(f'ink size: tablet {t[1]-t[0]:.1f} x {t[3]-t[2]:.1f} pt   device {(dx1-dx0)*SCALE:.1f} x {(dy1-dy0)*SCALE:.1f} pt at 72/226')
    print(f'  scale check: x {(t[1]-t[0])/((dx1-dx0)*SCALE):.4f}  y {(t[3]-t[2])/((dy1-dy0)*SCALE):.4f}')
    print(f'  implied offsets: Cx {t[0]-dx0*SCALE:.2f}pt  Cy {t[2]-dy0*SCALE:.2f}pt')
    print(f'  page half width is 480.00pt; page top would then sit at device y {-(t[2]-dy0*SCALE)/SCALE:.1f}px')

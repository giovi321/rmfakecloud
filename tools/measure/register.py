import numpy as np, pypdfium2 as pdfium

S = 2.0  # px per point when rasterising
tab = pdfium.PdfDocument('/cmp/lez-tablet.pdf')[0]
pay = pdfium.PdfDocument('/cmp/lez-payload.pdf')[7]
W, H = tab.get_size()

a = np.asarray(tab.render(scale=S).to_pil().convert('L'), dtype=np.int16)
p = np.asarray(pay.render(scale=S).to_pil().convert('L'), dtype=np.int16)

# the page's own top aligns with the tablet page top (measured earlier)
mask = np.zeros(a.shape, dtype=bool)
rows, cols = p.shape
mask[:rows, :cols] = np.abs(a[:rows, :cols] - p) > 60
if rows < a.shape[0]:
    mask[rows:, :] = a[rows:, :] < 200
print('tablet ink pixels:', int(mask.sum()), 'page', W, 'x', H)

pts = np.loadtxt('/cmp/points8.txt')
print('stroke points:', len(pts))

def score(scale, dx, dy):
    xs = (pts[:,0]*scale + dx) * S
    ys = (pts[:,1]*scale + dy) * S
    xi = np.rint(xs).astype(int); yi = np.rint(ys).astype(int)
    ok = (xi >= 0) & (xi < mask.shape[1]) & (yi >= 0) & (yi < mask.shape[0])
    if ok.sum() == 0: return 0.0
    return mask[yi[ok], xi[ok]].mean() * (ok.sum()/len(pts))

# centre the point cloud on the ink for each candidate scale, then refine
iy, ix = np.nonzero(mask)
inkcx, inkcy = ix.mean()/S, iy.mean()/S
pcx, pcy = pts[:,0].mean(), pts[:,1].mean()

best=None
for scale in np.arange(0.20, 0.80, 0.0025):
    dx0 = inkcx - pcx*scale
    dy0 = inkcy - pcy*scale
    for ddx in np.arange(-12, 12.1, 3):
        for ddy in np.arange(-12, 12.1, 3):
            s = score(scale, dx0+ddx, dy0+ddy)
            if best is None or s > best[0]:
                best = (s, scale, dx0+ddx, dy0+ddy)
print(f'coarse: overlap {best[0]:.3f} scale {best[1]:.4f} dx {best[2]:.2f} dy {best[3]:.2f}')

s0, sc, dx, dy = best
for _ in range(3):
    cand=[]
    for scale in np.arange(sc-0.01, sc+0.0101, 0.0005):
        for ddx in np.arange(dx-3, dx+3.01, 0.5):
            for ddy in np.arange(dy-3, dy+3.01, 0.5):
                cand.append((score(scale, ddx, ddy), scale, ddx, ddy))
    s0, sc, dx, dy = max(cand)
print(f'refined: overlap {s0:.3f}  scale {sc:.5f} pt/px  dx {dx:.2f}  dy {dy:.2f}')
print(f'  72/226 = {72/226:.5f}   ratio to it {sc/(72/226):.4f}')
print(f'  page width {W} / scale = {W/sc:.0f} device px')
print(f'  dx vs page centre offset: {dx:.2f}   (W/2 - 702*scale = {W/2 - 702*sc:.2f})')

import numpy as np, pypdfium2 as pdfium
from PIL import Image

S=2.0
tab = pdfium.PdfDocument('/cmp/lez-tablet.pdf')[0]
pay = pdfium.PdfDocument('/cmp/lez-payload.pdf')[7]
a = np.asarray(tab.render(scale=S).to_pil().convert('L'), dtype=np.int16)
p = np.asarray(pay.render(scale=S).to_pil().convert('L'), dtype=np.int16)
mask = np.zeros(a.shape, dtype=bool)
rows, cols = p.shape
mask[:rows,:cols] = np.abs(a[:rows,:cols]-p) > 60
if rows < a.shape[0]:
    mask[rows:,:] = a[rows:,:] < 200
Image.fromarray((~mask*255).astype('uint8')).save('/cmp/shape-tablet.png')

pts=np.loadtxt('/cmp/points8.txt')
h=int(1872*S/3); w=int(1404*S/3)
img=np.ones((h,w),dtype='uint8')*255
xi=np.rint(pts[:,0]*S/3).astype(int); yi=np.rint(pts[:,1]*S/3).astype(int)
ok=(xi>=0)&(xi<w)&(yi>=0)&(yi<h)
img[yi[ok],xi[ok]]=0
Image.fromarray(img).save('/cmp/shape-points.png')
print('tablet mask', mask.shape, 'ink px', int(mask.sum()))
print('points plotted in a 1404x1872 canvas:', int(ok.sum()), 'of', len(pts))

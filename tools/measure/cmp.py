import pypdfium2 as pdfium
from PIL import ImageChops
imgs={}
for n in ('align','final'):
    d=pdfium.PdfDocument(f'/check/cmp-{n}.pdf')
    print(n, 'pages', len(d), 'size', d[0].get_size())
    imgs[n]=[d[i].render(scale=1.0).to_pil().convert('L') for i in range(len(d))]
same=True
for i,(a,b) in enumerate(zip(imgs['align'], imgs['final'])):
    diff=ImageChops.difference(a,b)
    box=diff.getbbox()
    if box is not None:
        same=False
        print(f'  page {i+1} differs, bbox {box}')
print('pixel identical:', same)

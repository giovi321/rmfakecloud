import pypdfium2 as pdfium
from PIL import Image
t=pdfium.PdfDocument('/cmp/tablet.pdf')[0].render(scale=1.3).to_pil()
r=pdfium.PdfDocument('/cmp/rmfc.pdf')[3].render(scale=1.3).to_pil()
print('tablet', t.size, 'rmfakecloud', r.size)
w=max(t.size[0], r.size[0]); h=t.size[1]+r.size[1]+12
sheet=Image.new('RGB',(w,h),'white')
sheet.paste(t,(0,0)); sheet.paste(r,(0,t.size[1]+12))
sheet.save('/cmp/side.png')
print('top = tablet, bottom = rmfakecloud')

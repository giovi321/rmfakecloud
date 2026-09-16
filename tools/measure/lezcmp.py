import pypdfium2 as pdfium
from PIL import Image
t=pdfium.PdfDocument('/cmp/lez-tablet.pdf')[0].render(scale=1.3).to_pil()
n=pdfium.PdfDocument('/cmp/lez-new.pdf')[7].render(scale=1.3).to_pil()
w=max(t.size[0],n.size[0]); h=t.size[1]+n.size[1]+12
s=Image.new('RGB',(w,h),'white'); s.paste(t,(0,0)); s.paste(n,(0,t.size[1]+12))
s.save('/cmp/lezcmp.png'); print('top = tablet, bottom = new build', t.size, n.size)

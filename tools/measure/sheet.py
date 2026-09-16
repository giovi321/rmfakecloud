import pypdfium2 as pdfium
from PIL import Image
d=pdfium.PdfDocument('/check/cmp-final.pdf')
pages=[d[i].render(scale=0.65).to_pil() for i in range(len(d))]
w,h=pages[0].size
cols,rows=2,3
sheet=Image.new('RGB',(w*cols,h*rows),'white')
for i,p in enumerate(pages[:cols*rows]):
    sheet.paste(p,((i%cols)*w,(i//cols)*h))
sheet.save('/check/sheet.png')
print('contact sheet', sheet.size, 'from', len(pages), 'pages')

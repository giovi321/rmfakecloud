import pypdfium2 as pdfium
from PIL import Image
crops=[]
for n in ('portrait','landscape'):
    d=pdfium.PdfDocument(f'/check/m-{n}.pdf')
    img=d[2].render(scale=2.0).to_pil()      # page 3
    w,h=img.size
    crops.append(img.crop((0, int(h*0.25), int(w*0.55), int(h*0.75))))
w,h=crops[0].size
sheet=Image.new('RGB',(w, h*2+10),'white')
sheet.paste(crops[0],(0,0)); sheet.paste(crops[1],(0,h+10))
sheet.save('/check/p3.png')
print('top = portrait model, bottom = landscape model', sheet.size)

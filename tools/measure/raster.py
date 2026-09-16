import pypdfium2 as pdfium
for name in ('target','out-scale1','out-center','out-tl'):
    doc = pdfium.PdfDocument(f'/check/{name}.pdf')
    page = doc[0]
    img = page.render(scale=1.2).to_pil()
    img.save(f'/check/{name}.png')
    print(name, img.size)

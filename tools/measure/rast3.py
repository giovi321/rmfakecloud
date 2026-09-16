import pypdfium2 as pdfium
for n in ('gen-up','gen-down','gen-zero'):
    pdfium.PdfDocument(f'/check/{n}.pdf')[0].render(scale=1.2).to_pil().save(f'/check/{n}.png')
print('ok')

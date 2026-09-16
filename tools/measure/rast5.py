import pypdfium2 as pdfium
for n in ('top','center','bottom'):
    pdfium.PdfDocument(f'/check/gen-off-{n}.pdf')[0].render(scale=1.0).to_pil().save(f'/check/gen-off-{n}.png')
print('rendered')

import pypdfium2 as pdfium
d = pdfium.PdfDocument('/check/gen-ink1.pdf')
p = d[0]
print('ink page size (points):', p.get_size())
p.render(scale=1.5).to_pil().save('/check/gen-ink1.png')

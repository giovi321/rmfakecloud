import pypdfium2 as pdfium
d = pdfium.PdfDocument('/check/nb.pdf')
print('pages', len(d), 'page1 size', d[0].get_size())
d[0].render(scale=0.9).to_pil().save('/check/nb-p1.png')

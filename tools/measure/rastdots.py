import pypdfium2 as pdfium
d = pdfium.PdfDocument('/check/dots.pdf')
print('pages', len(d), 'size', d[0].get_size())
d[0].render(scale=1.1).to_pil().save('/check/dots-p1.png')

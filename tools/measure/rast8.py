import pypdfium2 as pdfium
d = pdfium.PdfDocument('/check/gen-align.pdf')
print('pages', len(d), 'size', d[0].get_size())
for i in range(min(2, len(d))):
    d[i].render(scale=1.2).to_pil().save(f'/check/align-p{i+1}.png')

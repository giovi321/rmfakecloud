import pypdfium2 as pdfium
doc = pdfium.PdfDocument('/check/gen-export.pdf')
for i in range(min(3, len(doc))):
    doc[i].render(scale=1.2).to_pil().save(f'/check/gen-p{i+1}.png')
print('rendered', min(3, len(doc)), 'pages')

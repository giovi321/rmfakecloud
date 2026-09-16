import pypdfium2 as pdfium
pdfium.PdfDocument('/check/gen-crop-fit.pdf')[0].render(scale=1.2).to_pil().save('/check/gen-crop-fit.png')
print('ok')

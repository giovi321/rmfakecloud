import pypdfium2 as pdfium
for n in ('top','center','bottom'):
    try:
        pdfium.PdfDocument(f'/check/gen2-{n}.pdf')[0].render(scale=1.0).to_pil().save(f'/check/gen2-{n}.png')
    except Exception as e:
        print(n, 'skip', e)
print('rendered')

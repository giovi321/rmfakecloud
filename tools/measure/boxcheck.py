import pypdfium2 as pdfium
for n in ('inside','wider','outside'):
    try:
        d=pdfium.PdfDocument(f'/check/box-{n}.pdf')
        print(f'{n:8} rendered size {d[0].get_size()}')
    except Exception as e:
        print(n, 'ERROR', e)

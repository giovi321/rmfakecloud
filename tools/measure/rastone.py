import pypdfium2 as pdfium
d=pdfium.PdfDocument('/check/one.pdf')
print('pages', len(d), 'size', d[0].get_size())
img=d[0].render(scale=1.4).to_pil()
img.save('/check/one.png')
# how dark is the page: count non-white pixels
import collections
px=img.convert('L').getdata()
dark=sum(1 for v in px if v < 200)
print('dark pixels:', dark, 'of', len(px), f'({100*dark/len(px):.3f}%)')

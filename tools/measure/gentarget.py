from pypdf import PdfReader, PdfWriter
r=PdfReader('/check/gen-payload.pdf'); w=PdfWriter(); w.add_page(r.pages[0])
with open('/check/gen-target1.pdf','wb') as f: w.write(f)
print('target page 1 extracted')
